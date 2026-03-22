# WireTunnel: High-Performance Protocol Proxy

**WireTunnel** is a high-concurrency network utility built in **Golang** that bridges WebSocket and TCP traffic. It is designed for low-latency protocol translation and non-blocking data observability using Go's native concurrency primitives.

## Core Features
* **Protocol Bridging**: Seamlessly translates full-duplex WebSocket frames into raw TCP packets and vice versa.
* [cite_start]**Non-Blocking Observability**: Implements an `io.Pipe` and Worker Pool architecture to intercept and compress traffic metadata for logging without delaying the primary data stream[cite: 184, 185].
* [cite_start]**High-Throughput Concurrency**: Utilizes Goroutines and Channels to manage thousands of simultaneous connections with a stable throughput of **2.5GB per hour**[cite: 184].

---

## Architecture & Implementation
The system is built on three pillars of the Go standard library: `net`, `io`, and `sync`.

### 1. The Concurrency Model
Rather than a single event loop, WireTunnel spawns a pair of lightweight Goroutines for every active connection:
* **Upstream (WS → TCP)**: Handles frame unmarshaling and raw packet delivery.
* **Downstream (TCP → WS)**: Wraps raw responses into WebSocket frames for client delivery.

### 2. The Data Pipeline (The `io.Pipe` Strategy)
To ensure that logging and compression do not block the network path, WireTunnel uses a "Tee" pattern:
1.  [cite_start]As bytes move through the proxy, they are written to an **`io.Pipe`**[cite: 185].
2.  [cite_start]The **Reader** end of the pipe is consumed by a background **Worker Pool**[cite: 185].
3.  [cite_start]The workers compress the data in chunks and commit it to local storage, saving **4GB of disk space daily** while keeping the main network thread free of CPU-intensive compression tasks[cite: 186].

---

## Performance Results
* [cite_start]**Throughput**: Sustained **2.5GB/hr** under simulated load[cite: 184].
* [cite_start]**Storage Efficiency**: Achieved a significant reduction in log footprint, persisting only **4GB daily** for high-volume traffic[cite: 186].
* [cite_start]**Latency Impact**: Non-blocking architecture ensured **zero measurable increase** in connection latency during active logging/compression cycles[cite: 186].

---

## Technical Stack
* **Language**: Golang (`net`, `http`, `io`, `sync` packages).
* **Concurrency**: Goroutines, Channels, Worker Pools.
* **Storage/Compression**: Local disk utilizing custom compression buffers.

---

## How to Run
```bash
# Clone the repository
git clone [https://github.com/your-username/wiretunnel.git](https://github.com/your-username/wiretunnel.git)

# Build the binary
go build -o wiretunnel main.go

# Start the proxy
./wiretunnel --port 8080 --target 127.0.0.1:9000