class WaitGroup
  def initialize(val)
    @val = val
    @mutex = Mutex.new
    @cond = ConditionVariable.new
  end

  def done
    @mutex.synchronize do
      raise 'WG below 0' if @val.negative?

      @val -= 1
      @cond.broadcast if @val.zero?
    end
  end

  def wait
    @mutex.synchronize { @cond.wait(@mutex) }
  end
end


