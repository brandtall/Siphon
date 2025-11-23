package main

import (
	"encoding/json"
	"net"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var PacketsReceived = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "siphon_packets_received_total",
	Help: "Total UDP packets received",
})

var bufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 32*1024)
		return &b
	},
}

type Packet struct {
	Data   *[]byte
	Length int
}

type LogEntry struct {
	Service string `json:"service"`
	Level   string `json:"level"`
	Msg     string `json:"msg"`
}

func reader(conn net.PacketConn, packetChan chan Packet) {
	for {
		bufPtr := bufferPool.Get().(*[]byte)
		n, _, err := conn.ReadFrom(*bufPtr)
		if err != nil {
			bufferPool.Put(bufPtr)
			continue
		}

		PacketsReceived.Inc()

		select {
		case packetChan <- Packet{Data: bufPtr, Length: n}:
		default:
			bufferPool.Put(bufPtr)
		}
	}
}

func worker(packetChan chan Packet, entryChan chan LogEntry) {
	for pkg := range packetChan {
		var entry LogEntry
		err := json.Unmarshal((*pkg.Data)[:pkg.Length], &entry)

		bufferPool.Put(pkg.Data)

		if err != nil {
			continue
		}

		entryChan <- entry
	}
}

func batchProcessor(entryChan chan LogEntry, producer LogProducer) {
	const BatchSize = 1000
	
	batch := make([]LogEntry, 0, BatchSize)
	
	for {
		select {
		case entry := <-entryChan:
			batch = append(batch, entry)
			if len(batch) >= BatchSize {
				producer.SendBatch(batch)
				batch = batch[:0]
			}
		}
	}
}
