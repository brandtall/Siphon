package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
)

var rowsInserted = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "siphon_rows_inserted_total",
	Help: "Total rows inserted into ClickHouse",
})

type LogEntry struct {
	Service string `json:"service"`
	Level   string `json:"level"`
	Msg     string `json:"msg"`
}

func main() {
	prometheus.MustRegister(rowsInserted)
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":2113", nil)
	}()

	time.Sleep(10 * time.Second)

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"clickhouse:9000"},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: "default",
			Password: "password",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	if err := conn.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS logs (
			timestamp DateTime DEFAULT now(),
			service String,
			level String,
			msg String
		) ENGINE = MergeTree()
		ORDER BY timestamp
	`)
	if err != nil {
		log.Fatal(err)
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"kafka:9092"},
		Topic:    "siphon-logs",
		GroupID:  "clickhouse-ingester",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})
	defer r.Close()

	batchSize := 1000
	batch := make([]LogEntry, 0, batchSize)
	ticker := time.NewTicker(1 * time.Second)

	for {
		m, err := r.FetchMessage(ctx)
		if err != nil {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal(m.Value, &entry); err == nil {
			batch = append(batch, entry)
		}

		select {
		case <-ticker.C:
			if len(batch) > 0 {
				flush(ctx, conn, batch)
				r.CommitMessages(ctx, m)
				batch = batch[:0]
			}
		default:
			if len(batch) >= batchSize {
				flush(ctx, conn, batch)
				r.CommitMessages(ctx, m)
				batch = batch[:0]
			}
		}
	}
}

func flush(ctx context.Context, conn clickhouse.Conn, batch []LogEntry) {
	batchOp, err := conn.PrepareBatch(ctx, "INSERT INTO logs (service, level, msg)")
	if err != nil {
		return
	}

	for _, e := range batch {
		if err := batchOp.Append(e.Service, e.Level, e.Msg); err != nil {
			continue
		}
	}

	if err := batchOp.Send(); err == nil {
		rowsInserted.Add(float64(len(batch)))
	}
}
