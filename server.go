package main

import (
	"fmt"
	"io"
	"log"
	"net"
)

type Server struct {
	listenerAddress string
}

func (server *Server) Init(listenerAddress string) {
	server.listenerAddress = listenerAddress
}

func (server *Server) Start() {
	listener, err := net.Listen("tcp", server.listenerAddress)
	if err != nil {
		log.Fatal("Error listening:", err)
	}

	// loop for every incoming connection to server
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting conn:", err)
			continue
		}

		fmt.Printf("Server starting a goroutine...\n")
		go server.handleConnection(conn)
	}
}

func (server *Server) handleConnection(conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("failed to close connection: %v", err)
		}
	}()

	// data from client
	packet := make([]byte, 0)
	tmp := make([]byte, 4096)

	// read loop through connection
	for {
		n, err := conn.Read(tmp)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Println("Read error:", err)
			return
		}
		packet = append(packet, tmp[:n]...)
	}

	num, err := conn.Write(packet)
	if err != nil {
		log.Println("Write error:", err)
	}
	fmt.Printf("Server received %d bytes, the payload is %s\n", num, string(packet))
}
