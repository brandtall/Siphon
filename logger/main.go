package main

import (
	"context"
	"log"
	"net"
	"runtime"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	kafkaURL := "kafka:9092"
	topic := "siphon-logs"

	for i := 0; i < 30; i++ {
		conn, err := kafka.DialLeader(context.Background(), "tcp", kafkaURL, topic, 0)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(1 * time.Second)
	}

	kafkaWriter := NewKafkaProducer([]string{kafkaURL}, topic)
	defer kafkaWriter.Close()

	conn, err := net.ListenPacket("udp", ":5050")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	packetChan := make(chan Packet, 2000)
	entryChan := make(chan LogEntry, 5000)

	go reader(conn, packetChan)

	numWorkers := runtime.NumCPU()
	for i := 0; i < numWorkers; i++ {
		go worker(packetChan, entryChan)
	}

	go batchProcessor(entryChan, kafkaWriter)

	select {}
}
