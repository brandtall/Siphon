package main

import (
	"context"
	"encoding/json"
	"time"
	"github.com/segmentio/kafka-go"
)

type LogProducer interface {
	SendBatch(batch []LogEntry) error
	Close() error
}

type KafkaProducer struct {
	writer *kafka.Writer
}

func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
	}
	return &KafkaProducer{writer: w}
}

func (k *KafkaProducer) SendBatch(batch []LogEntry) error {
	msgs := make([]kafka.Message, len(batch))

	for i, entry := range batch {
		jsonBytes, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		msgs[i] = kafka.Message{
			Value: jsonBytes,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return k.writer.WriteMessages(ctx, msgs...)
}

func (k *KafkaProducer) Close() error {
	return k.writer.Close()
}
