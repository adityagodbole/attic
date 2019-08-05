package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

type RedisSync struct {
	redis *redis.Client
}

func newRedisSync() *RedisSync {
	return &RedisSync{
		redis: redis.NewClient(&redis.Options{}),
	}
}

func (r *RedisSync) Set(id, stage string, res *StageResult) error {
	syncData, err := r.GetAll(id)
	if err != nil {
		return err
	}
	syncData[stage] = res
	jsonVal, err := json.Marshal(syncData)
	if err != nil {
		return errors.Wrap(err, "Unable to marshall stage data")
	}
	r.redis.Set(r.makeKey(id), jsonVal, 0)
	return nil
}

func (r *RedisSync) Get(id, stage string) (*StageResult, error) {
	data, err := r.GetAll(id)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting stage data")
	}
	stageData := data[stage]
	return stageData, nil
}

func (r *RedisSync) GetAll(id string) (map[string]*StageResult, error) {
	data := make(map[string]*StageResult)
	v, err := r.redis.Get(r.makeKey(id)).Result()
	if err != nil {
		return data, nil // return empty map if not found
	}
	bytes := []byte(v)
	if err != nil {
		return nil, errors.Wrap(err, "Cannot fetch all data for id:"+id)
	}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, errors.Wrap(err, "Cannot unmarshal stages data")
	}
	return data, nil
}
func (r *RedisSync) makeKey(s string) string {
	return "stages-" + s
}

type Syncer interface {
	Set(id, stage string, result *StageResult) error
	Get(id, stage string) (*StageResult, error)
	GetAll(id string) (map[string]*StageResult, error)
}

type StageResult struct {
	Data   interface{}
	Status string
}

type ResultWriter interface {
	Set(data interface{})
}
type StageHandler func(rw ResultWriter, data interface{}, seed interface{}) error

type Stage struct {
	name string
	//	result  *StageResult
	handler StageHandler
}

type StageResultWriter struct {
	result   StageResult
	bookmark bool
}

func (st *StageResultWriter) Set(data interface{}) {
	st.result.Data = data
}

func (st *StageResultWriter) Bookmark(data interface{}) {
	st.result.Data = data
	st.bookmark = true
}

type Stages struct {
	syncer     Syncer
	stages     []*Stage
	lastResult *StageResult
}

func Init(syncer Syncer) *Stages {
	return &Stages{
		syncer: syncer,
		stages: []*Stage{},
	}
}

func (st *Stages) Then(name string, fn StageHandler) *Stages {
	stage := Stage{
		name:    name,
		handler: fn,
	}
	st.stages = append(st.stages, &stage)
	return st
}

func (st *Stages) Add(name string, fn StageHandler) *Stages {
	return st.Then(name, fn)
}

func (st *Stages) Run(id string, seed interface{}) error {
	syncData, err := st.syncer.GetAll(id)
	if err != nil {
		return errors.Wrap(err, "Cannot fetch synced data")
	}
	prevResult := &StageResult{Data: seed}
	for _, stage := range st.stages {
		syncedStageResult, ok := syncData[stage.name]
		if !ok {
			syncedStageResult = &StageResult{}
		}
		if syncedStageResult.Status == "done" {
			prevResult = syncedStageResult
			continue
		}
		stageResultWriter := &StageResultWriter{}
		if err := stage.handler(stageResultWriter, prevResult.Data, seed); err != nil {
			return err
		}
		stageResult := stageResultWriter.result
		stageResult.Status = "done"
		prevResult = &stageResult
		if err := st.syncer.Set(id, stage.name, &stageResult); err != nil {
			return errors.Wrap(err, "Cannot sync state result")
		}
		if stageResultWriter.bookmark {
			break
		}
	}
	st.lastResult = prevResult
	return nil
}

func (st *Stages) Result() interface{} {
	return st.lastResult.Data
}

var testSt *Stages
var testOnce sync.Once

func makeTestStages(syncer Syncer) *Stages {
	return Init(syncer).
		Then("first", func(rw ResultWriter, data, seed interface{}) error {
			log.Println("executing first stage")
			str, ok := data.(string)
			if !ok {
				return errors.New("Cannot read input data")
			}
			rw.Set(str + "first")
			return nil
		}).
		Then("second", func(rw ResultWriter, data, seed interface{}) error {
			log.Println("executing second stage")
			str, ok := data.(string)
			if !ok {
				return errors.New("Cannot read input data")
			}
			rw.Set(str + "second")
			return nil
		}).
		Then("third", func(rw ResultWriter, data, seed interface{}) error {
			log.Println("executing third stage")
			str, ok := data.(string)
			if !ok {
				return errors.New("Cannot read input data")
			}
			rw.Set(str + "third")
			return nil
		})
}

func testStages(syncer Syncer) *Stages {
	testOnce.Do(func() {
		testSt = makeTestStages(syncer)
	}) // cache
	return testSt // and return
}

func main() {
	st := testStages(newRedisSync())
	err := st.Run("random-id1", "seed")
	if err != nil {
		log.Printf("Top level error = %+v\n", err)
		return
	}
	fmt.Printf("result = %+v\n", st.Result())
}
