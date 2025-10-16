package main

import (
	"log"
	"net"
	"termsg/server_state"
)

const IP = "localhost:8080"

func main() {
	var err error
	Server_state := new(types.Server)
	Server_state.Listener, err = net.Listen("tcp", IP)
	if err != nil {
		log.Fatal(err)
	}
	Server_state.Server_main()
}
