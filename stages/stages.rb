# frozen_string_literal: true

require "json"
require "redis"

# class to sync stages to a file
class FileSync
  def set(id, stage, result, status)
    sync_data = get_all(id)
    sync_data[stage] = { "result" => result, "status" => status }
    File.write(id, JSON.generate(sync_data))
  end

  def get(id, stage)
    get_all(id)[stage]
  end

  def get_all(id)
    data = File.read(id)
    JSON.parse(data)
  rescue StandardError
    {}
  end
end

Bookmark = Struct.new(:data)
StageData = Struct.new(:name, :result, :blk, :ttl, keyword_init: true)
StageResult = Struct.new(:data, :status, keyword_init: true)

# class to sync stages to redis
class RedisSync
  def initialize
    @redis = Redis.new
  end

  def set(id, stage)
    sync_data = get_all(id)
    sync_data[stage.name] = stage.result.to_h
    key = make_key(id)
    @redis.set(key, JSON.generate(sync_data))
  end

  def get(id, name)
    get_all(id)[name]
  end

  def get_all(id)
    data = @redis.get(make_key(id))
    JSON.parse(data, symbolize_names: true)
  rescue StandardError
    {}
  end

  def lock(id, stage)
    key = make_lock_key(id, stage.name)
    if @redis.set(key, true, ns: true, ex: stage.ttl)
      return true
    else
      false
    end
  end

  def unlock(id, stage)
    @redis.del(make_lock_key(id, stage.name))
  end

  private

  def make_lock_key(id, name)
    "stages-#{id}-#{name}-lock"
  end

  def make_key(id); "stages-#{id}"; end
end

class StageConflictError < StandardError
  attr_reader :ttl
  def initialize(ttl, stage = nil)
    @ttl = ttl
    @stage = stage
  end

  def to_s
    "stage conflict: #{@stage}: #{@ttl} seconds"
  end
end

# Implements stages execution logic
class Stages
  def initialize(syncer)
    @syncer = syncer
    @stages = []
  end

  def stage(name, time_hint, &blk)
    stage_data = StageData.new(name: name, blk: blk,
       ttl: time_hint, result: StageResult.new(status: "init"))
    @stages.push stage_data
    self
  end
  alias_method :then, :stage

  def run(id, seed = nil)
    sync_data = @syncer.get_all(id) || {}
    merged_stages = @stages.map do |stage|
      sync_stage = sync_data[stage.name.to_sym] || {}
      stage.result = StageResult.new(**sync_stage)
      stage
    end
    run_stages(merged_stages, id, seed)
  end
  
  private

  def run_stages(stages, id, seed = nil)
    stages.reduce(seed) do |input, stage|
      if stage.result.status == "done"
        stage.result.data
      else
        res = exec_stage(id, stage, input, seed)
        if res.is_a? Bookmark
          break res
        else
          next res
        end
      end
    end
  end

  def exec_stage(id, stage, input, seed)
    unless @syncer.lock(id, stage)
      raise StageConflictError.new(stage.ttl / 2)
    end
    blk = stage.blk
    raise "Cannot find handler for stage #{stage.name}" unless blk

    res = blk.call(input, seed)
    sync_res = if res.is_a? Bookmark
                 res.data
                else
                  res
                end
    stage.result = StageResult.new(data: sync_res, status: "done")
    @syncer.set(id, stage)
    res
  ensure
    @syncer.unlock(id, stage)
  end
end

def staged1
  Stages.new(RedisSync.new)
    .then("first", 1) { |data|
      p "doing first"
      # fail "fail at stage 1"
      data + "foo"
    }.then("second", 1) { |data|
      p "doing second"
      # fail "fail at stage 2"
      data + "bar"
    }.then("third", 1) { |data|
      p "doing third"
      # fail "fail at stage 3"
      data + "third"
    }.run("random-id1", "seed")
end

def staged2
  Stages.new(RedisSync.new)
    .then("first", 1) { |data|
      p "doing first"
      Bookmark.new(data + "foo")
    }.then("second", 1) { |_, seed|
      p "doing second"
      Bookmark.new(seed + "bar")
    }.then("third", 1) { |data, _|
      p "doing third"
      data + "third"
    }.run("random-id1", "seed")
end

p staged2
