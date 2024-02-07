package main

import (
	"fmt"
	"net"
)

func main() {
	server()
}

func server() {
	conn, err := net.ListenPacket("udp", ":51820")
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 1024)

	for {
		fmt.Println("wait for data")
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			panic(err)
		}

		if n == 0 {
			continue
		}

		host, port, err := net.SplitHostPort(addr.String())
		if err != nil {
			panic(err)
		}

		fmt.Printf("Getting data from %s:%s='%s'\n", host, port, string(buf[:n]))
	}
}
