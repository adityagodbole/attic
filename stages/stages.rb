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

# class to sync stages to redis
class RedisSync
  def initialize
    @redis = Redis.new
  end

  def set(id, stage, result, status)
    sync_data = get_all(id)
    sync_data[stage] = { "result" => result, "status" => status }
    key = make_key(id)
    @redis.set(key, JSON.generate(sync_data))
  end

  def get(id, stage)
    get_all(id)[stage]
  end

  def get_all(id)
    data = @redis.get(make_key(id))
    JSON.parse(data)
  rescue StandardError
    {}
  end

  private

  def make_key(id); "stages-#{id}"; end
end

Bookmark = Struct.new(:data)

# Implements stages execution logic
class Stages
  def initialize(id, syncer, seed = nil)
    @id = id
    @syncer = syncer
    @stages = []
    @seed = seed
  end

  def stage(name, &blk)
    stage_data = { "name" => name, "status" => "init", "blk" => blk }
    @stages.push stage_data
    self
  end

  def do
    sync_data = @syncer.get_all(@id) || {}
    merged_stages = @stages.map do |stage|
      sync_stage = sync_data[stage["name"]] || {}
      stage.merge(sync_stage)
    end
    run_stages(merged_stages, @seed)
  end

  private

  def run_stages(stages, seed)
    stages.reduce(seed) do |input, stage|
      if stage["status"] == "done"
        stage["result"]
      else
        res = exec_stage(stage, input)
        if res.is_a? Bookmark
          @syncer.set(@id, name, res.data, "done")
          break res
        else
          @syncer.set(@id, name, res, "done")
          next res
        end
      end
    end
  end

  def exec_stage(stage, input)
    blk = stage["blk"]
    name = stage["name"]
    raise "Cannot find handler for stage #{name}" unless blk

    blk.call(input, seed)
  end
end

def staged1
  stager = Stages.new("random-id1", RedisSync.new, "seed")
  stager.stage("first") { |data|
    p "doing first"
    # fail "fail at stage 1"
    data + "foo"
  }.stage("second") { |data|
    p "doing second"
    # fail "fail at stage 2"
    data + "bar"
  }.stage("third") { |data|
    p "doing third"
    # fail "fail at stage 3"
    data + "third"
  }.do
end

def staged2
  stager = Stages.new("random-id1", RedisSync.new, "seed")
  stager.stage("first") { |data|
    p "doing first"
    Bookmark.new(data + "foo")
  }.stage("second") { |_, seed|
    p "doing second"
    Bookmark.new(seed + "bar")
  }.stage("third") { |data, _|
    p "doing third"
    data + "third"
  }.do
end

p staged2
