require 'socket'
require_relative 'wg'

# Wrapper over TCPSocket
class SSocket < TCPSocket
  def do_read(max_bytes = 1024, &rdcb)
    @readcb = rdcb
    @max_bytes = max_bytes
    @reactor.sched_read(self)
  end

  def bind_reactor(reactor)
    @reactor = reactor
    self
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
    @active = false
  end

  def run
    @active = true
    loop do
      break unless @active

      readers = []
      readers << @readq.pop until @readq.empty?
      if readers.empty?
        sleep 1
        next
      end
      rds, = IO.select(readers)

      rds&.each { |r| r.read_and_callback }
      remaining = readers - rds
      remaining.each { |r| sched_read(r) }
    end
  end
end

def selector
  count = 100
  wg = WaitGroup.new(count)
  r = Selector.new
  socks = (1..count).map { r.new_connection('127.0.0.1', '8888') }
  socks.each do |sock|
    sock.puts "hello"
    sock.do_read do |resp|
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
  sleep 10
end

def blocking
  threads = (1..100).map do
    sock = TCPSocket.new('127.0.0.1', '8888')
    sock.puts "hello"
    Thread.new do
      data = sock.recv(64)
      puts data
      sleep 10
    end
  end
  threads.each(&:join)
end

#blocking
selector
