package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

/////// Syncer /////////////////

type Syncer interface {
	Set(id, stage string, result *StageResult) error
	Get(id, stage string) (*StageResult, error)
	GetAll(id string) (map[string]*StageResult, error)
	Lock(id, stage string, ttl time.Duration) (bool, error)
	Unlock(id, stage string) error
}
type RedisSyncer struct {
	redis *redis.Client
}

func newRedisSyncer() *RedisSyncer {
	return &RedisSyncer{
		redis: redis.NewClient(&redis.Options{}),
	}
}

func (r *RedisSyncer) Set(id, stage string, res *StageResult) error {
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

func (r *RedisSyncer) Get(id, stage string) (*StageResult, error) {
	data, err := r.GetAll(id)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting stage data")
	}
	stageData := data[stage]
	return stageData, nil
}

func (r *RedisSyncer) GetAll(id string) (map[string]*StageResult, error) {
	data := make(map[string]*StageResult)
	v, err := r.redis.Get(r.makeKey(id)).Result()
	if err != nil {
		return nil, errors.Wrap(err, "Cannot fetch all data for id:"+id)
	}
	bytes := []byte(v)

	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, errors.Wrap(err, "Cannot unmarshal stages data")
	}
	return data, nil
}

func (r *RedisSyncer) Lock(id, stage string, ttl time.Duration) (bool, error) {
	key := r.makeKey(id)
	res := r.redis.SetNX(key, true, ttl)
	return true, res.Err()
}

func (r *RedisSyncer) Unlock(id, stage string) error {
	key := r.makeKey(id)
	res := r.redis.Del(key)
	return res.Err()
}

func (r *RedisSyncer) makeKey(s string) string {
	return "stages-" + s
}

/////////////// Syncer end /////////////////////

////////////// Stage Result ///////////////////
type StageResult struct {
	Data   interface{}
	Status string
}

type ResultWriter interface {
	Set(data interface{})
	Bookmark(data interface{})
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

////////// Stage result end ////////////////

////////// Single Stage /////////////////
type StageHandler func(rw ResultWriter, data interface{}, seed interface{}) error

type Stage struct {
	name string
	//	result  *StageResult
	handler StageHandler
	ttl     time.Duration
}

type StageConflictError error

// executes stage and returns stageResult, bookmark and error
func (stage *Stage) execStage(syncer Syncer, id string, input, seed interface{}) (*StageResult, bool, error) {
	// Take exclusive lock
	ok, err := syncer.Lock(id, stage.name, stage.ttl)
	if err != nil {
		return nil, false, StageConflictError(err)
	}
	if !ok {
		return nil, false, StageConflictError(fmt.Errorf("cannot get exclusive lock for %s:%s", stage.name, id))
	}
	defer func() {
		if err := syncer.Unlock(id, stage.name); err != nil {
			log.Printf("Cannot unset execution lock %+v", err)
		}
	}()

	// execute stage
	stageResultWriter := &StageResultWriter{}
	if err := stage.handler(stageResultWriter, input, seed); err != nil {
		return nil, false, err
	}
	stageResult := stageResultWriter.result
	stageResult.Status = "done"
	// sync stage data
	if err := syncer.Set(id, stage.name, &stageResult); err != nil {
		return nil, false, errors.Wrap(err, "Cannot sync state result")
	}
	return &stageResult, stageResultWriter.bookmark, nil
}

//////////// Single stage end ///////////

////// Stages ////////////////////

// Stages implements the functionality of idempotent stages
type Stages struct {
	syncer     Syncer
	stages     []*Stage
	lastResult *StageResult
}

// Initialise a Stages struct
func Init(syncer Syncer) *Stages {
	return &Stages{
		syncer: syncer,
		stages: []*Stage{},
	}
}

// Then adds an idempotent step to a Stages object
func (st *Stages) Then(name string, ttl time.Duration, fn StageHandler) *Stages {
	stage := Stage{
		name:    name,
		handler: fn,
		ttl:     ttl,
	}
	st.stages = append(st.stages, &stage)
	return st
}

// Add is an alias for Then
func (st *Stages) Add(name string, ttl time.Duration, fn StageHandler) *Stages {
	return st.Then(name, ttl, fn)
}

// Run executes the correct stage sequence depending on the current saves state
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
		stageResult, bookmark, err := stage.execStage(st.syncer, id, prevResult.Data, seed)
		if err != nil {
			return errors.Wrapf(err, "Unable to execute stage %s:%s", stage.name, id)
		}
		prevResult = stageResult
		if bookmark { // Bookmark is an explicit termination to be resumed later
			break
		}
	}
	st.lastResult = prevResult
	return nil
}

func (st *Stages) Result() interface{} {
	return st.lastResult.Data
}

/////////// Stages end //////////

//////// Example /////////
var testSt *Stages
var testOnce sync.Once

func prepareTestStages(syncer Syncer) *Stages {
	return Init(syncer).
		Then("first", 1*time.Second, func(rw ResultWriter, data, seed interface{}) error {
			log.Println("executing first stage")
			str, ok := data.(string)
			if !ok {
				return errors.New("Cannot read input data")
			}
			rw.Set(str + "first")
			return nil
		}).
		Then("second", 1*time.Second, func(rw ResultWriter, data, seed interface{}) error {
			log.Println("executing second stage")
			str, ok := data.(string)
			if !ok {
				return errors.New("Cannot read input data")
			}
			rw.Set(str + "second")
			return nil
		}).
		Then("third", 1*time.Second, func(rw ResultWriter, data, seed interface{}) error {
			log.Println("executing third stage")
			str, ok := data.(string)
			if !ok {
				return errors.New("Cannot read input data")
			}
			rw.Set(str + "third")
			return nil
		})
}

func makeTestStages(syncer Syncer) *Stages {
	testOnce.Do(func() {
		testSt = prepareTestStages(syncer)
	}) // cache
	return testSt // and return
}

func main() {
	st := makeTestStages(newRedisSyncer())
	err := st.Run("random-id1", "seed")
	if err != nil {
		log.Printf("Top level error = %+v\n", err)
		return
	}
	fmt.Printf("result = %+v\n", st.Result())
}
