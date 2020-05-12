require 'socket'
require_relative 'wg'

# Wrapper over TCPSocket
class SSocket < TCPSocket
  def bind_reactor(reactor)
    @reactor = reactor # bind the socket to the reactor
    self
  end

  def async_read(max_bytes = 1024, &rdcb)
    @readcb = rdcb # store reference to the callback
    @max_bytes = max_bytes
    @reactor.sched_read(self) # schedule a read call on the reactor
  end

  def read_and_callback
    data = recv_nonblock(@max_bytes)
    @readcb.call(data)
  end
end

# select loop
class Selector
  def initialize
    @readq = Queue.new
    @active = false
  end

  def new_connection(host, port)
    SSocket.new(host, port).bind_reactor(self)
  end

  def sched_read(sock)
    @readq.push(sock)
  end

  def stop
    @active = false # set flag to stop run loop
  end

  def run
    @active = true
    loop do
      break unless @active # check if we have received a stop

      readers = []
      readers << @readq.pop until @readq.empty? # get all read requests from queue

      if readers.empty?
        sleep 1
        next
      end

      readylist, = IO.select(readers) # wait till some readers have data

      readylist&.each { |sock| sock.read_and_callback }

      # reschedule readers who don't have data
      remaining = readers - readylist
      remaining.each { |r| sched_read(r) }
    end
  end
end

def selector(count)
  wg = WaitGroup.new(count)
  r = Selector.new
  socks = (1..count).map { r.new_connection('127.0.0.1', '8888') }
  socks.each do |sock|
    sock.puts "hello"
    sock.async_read do |resp|
      puts resp
      wg.done
    end
  end
  Thread.new do
    wg.wait
    $stderr.puts 'terminating reactor'
    r.stop
  end
  r.run
end

selector(ARGV[0].to_i)

