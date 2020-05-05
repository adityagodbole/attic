require 'socket'
 
class EchoServer
  def initialize(host='127.0.0.1', port=8888)
    @host = host
    @port = port
    @server = TCPServer.open(host, port)
  end
  def start_message
    STDOUT.puts "The server is running on #{@host}:#{@port}"
    STDOUT.puts "Press CTL-C to terminate"
  end
  def serve_forever
    start_message
    while client = @server.accept
      line = client.gets
      client.puts line
      client.close
    end
  end
end

echod = EchoServer.new
echod.serve_forever
