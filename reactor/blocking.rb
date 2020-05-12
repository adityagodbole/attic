# What is a ruby thread?

# Maps to a OS thread. One OS thread per ruby thread
# 8 kb of stack memory is locked in the kernel per thread.
#   This is non swappable memory. Locked in RAM
# Has its own user-space stack
# Some memory for ruby state around the OS thread
#
#
# States: Running, Sleep Interruptable, Sleep uninteruptable, Stopped
#     Running:                Active thread, utilizing CPU
#     Sleep interuptable:   Sleeping on event. Other Runnable threads can be scheduled
#     Sleep UnInterruptable:    Sleeping on IO, eg read call
#     Runnable:               Ready to be run. Read call has returned data
# read call puts thread in Sleep Uninteruptable
#
#
# Ruby GIL: Only one thread allowed in Running state
#           Any number allowed in other states - eg, blocking on a read call



require 'socket'

def blocking(count)
  threads = (1..count).map do
    sock = TCPSocket.new('127.0.0.1', '8888') # create new socket
    sock.puts "hello" # send data
    Thread.new do # create new thead (returned from block)
      data = sock.recv(64) # read data, thread in 'Sleep Uninteruptable'
      puts data # print it
      sleep 1 # simulate long read
    end
  end

  threads.each(&:join) # join waits for a thread,
                       # this line waits for all to finish
end

blocking(ARGV[0].to_i)
