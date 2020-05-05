package main

import (
	"io"
	"log"
	"net"
)

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:8888")
	if err != nil {
		log.Fatal("tcp server listener error:", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("tcp server accept error", err)
		}

		go io.Copy(conn, conn)
	}
}
