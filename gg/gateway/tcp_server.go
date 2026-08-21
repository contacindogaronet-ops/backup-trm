package gateway

import (
	"fmt"
	"net"
)

func StartTCPServer(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("[FATAL] TCP Listener jebol di %s: %v\n", addr, err)
		return
	}
	for {
		conn, err := ln.Accept()
		if err != nil { continue }
		go handleTCPConnection(conn)
	}
}
