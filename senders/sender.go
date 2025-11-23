package main 

import (
	"net"
)

func main() {
	conn, _ := net.Dial("udp", "siphon-logger:5050")
	msg := []byte(`{"service":"api","level":"info","msg":"User login"}`)

	for {
		conn.Write(msg)
	}
}
