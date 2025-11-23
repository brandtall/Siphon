# Siphon: High-Velocity UDP Log Aggregation

Peaks of 325k+ logs ingested per second.

## Performance

<img width="964" height="374" alt="image" src="https://github.com/user-attachments/assets/e850aebd-983c-42fe-8447-0162e7e59f8e" /> 

```shell
udp-sender  | 2025/11/23 10:38:23 Generating: 154465 logs/sec
udp-sender  | 2025/11/23 10:38:24 Generating: 256775 logs/sec
udp-sender  | 2025/11/23 10:38:25 Generating: 293692 logs/sec
udp-sender  | 2025/11/23 10:38:26 Generating: 250589 logs/sec
udp-sender  | 2025/11/23 10:38:27 Generating: 325140 logs/sec
```

This project has three parts:

1.  **The Logger:** A concurrent Go service using a Fan-Out pattern to maximize throughput. While standard UDP readers often bottleneck on the syscall loop, we decouple ingestion from processing. A dedicated reader loop captures raw packets and immediately dispatches them to N worker goroutines via buffered channels. This prevents blocking at the network layer during JSON parsing and Kafka batching operations.
2.  **The Buffer (Kafka):** Our Durable WAL (Write-Ahead Log) allowing the logger layer to operate incredibly fast while not overwhelming our storage.
3.  **The Ingester (Storage Writer):** Go consumer group on the other side reading from Kafka and bulk inserting into ClickHouse.

# Technical Deep Dive

## Concurrency Pattern

We utilize a single-producer, multi-consumer model to handle high-velocity UDP traffic. The main thread is dedicated solely to reading packets from the socket. To prevent packet loss during latency spikes, we employ Application-Side Shedding. If the worker channels are saturated, the reader drops the packet immediately. This intentional trade-off preserves the liveliness of the listener loop and adheres to the "fire-and-forget" nature of UDP.

## Resource Management

At 100k+ packets per second, allocating a new byte buffer for every payload generates massive Garbage Collection pressure. This constant allocation and cleanup steals valuable CPU cycles from the ingestion process. We solve this by implementing a Zero-Allocation Hot Path using sync.Pool. We recycle byte slices to amortize the cost of memory management, ensuring the heap remains stable under heavy load.

## Trade-offs

1.  By using sync.Pool, we are susceptible to an allocation spike after GC picks up, as it will clear out the pool and we will have to reallocate the buffers.
2.  We chose ClickHouse over Elasticsearch because for aggregation, Elasticsearch's word indexing is overkill and would cause a performance overhead. ClickHouse's MergeTree engine is a perfect fit here.
3.  Additionally, we are optimizing for "Count X over Time Y" queries which ClickHouse specializes in.

# Getting Started

This project is fully containerized using Docker Compose.

### Prerequisites

  * Docker & Docker Compose
  * Go 1.25 (optional, for local tool execution)

### 1\. Start Infrastructure

Spin up Zookeeper, Kafka, and ClickHouse.

```bash
docker-compose up -d zookeeper kafka clickhouse
```

### 2\. Start the Pipeline

Start the Logger (UDP Server) and Ingester (ClickHouse Writer).

```bash
docker-compose up -d siphon-logger siphon-ingester
```

### 3\. Generate Load

Start the high-velocity UDP packet generator.

```bash
docker-compose up -d udp-sender
```

### 4\. Verify Data

Query ClickHouse directly to see the row count increasing in real time.

```bash
echo "SELECT count(*) FROM logs" | curl 'http://default:password@localhost:8123/' --data-binary @-
```
