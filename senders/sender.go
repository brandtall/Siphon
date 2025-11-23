package main

import (
	"log"
	"net"
	"sync/atomic"
	"time"
)

func main() {
	conn, _ := net.Dial("udp", "siphon-logger:5050")
	msg := []byte(`{"service":"api","level":"info","msg":"User login transaction complete"}`)

	var ops uint64 = 0

	go func() {
		var lastOps uint64 = 0
		for range time.Tick(1 * time.Second) {
			curr := atomic.LoadUint64(&ops)
			log.Printf("Generating: %d logs/sec", curr-lastOps)
			lastOps = curr
		}
	}()

	for {
		conn.Write(msg)
		atomic.AddUint64(&ops, 1)
	}
}
