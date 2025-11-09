# Distributed Counter

A distributed counter implementation in Go that provides a scalable and fault-tolerant way to maintain counters across multiple nodes.

## Prerequisites

- Go 1.25 or higher
- Network connectivity between nodes

## Project Structure

```
distributedCounter/
├── go.mod
├── main.go
├── internal/
│   ├── api/
│   │   └── server.go
│   ├── counter/
│   │   └── counter.go
│   └── discovery/
│       └── discovery.go
└── models/
    └── message.go
```

## Getting Started

1. Clone the repository:
```bash
git clone https://github.com/Prasang-money/distributedCounter.git
cd distributedCounter
```

2. Install dependencies:
```bash
go mod download
```

### Run a 3-Node Cluster

Open 3 terminal windows:

**Terminal 1 - Node 1 (Bootstrap):**
```bash
make run-node1
# or
go run main.go --port=8080 --id=localhost:8080
```

**Terminal 2 - Node 2:**
```bash
make run-node2
# or
go run main.go --port=8081 --id=localhost:8081 --peers=localhost:8080
```

**Terminal 3 - Node 3:**
```bash
make run-node3
# or
go run main.go --port=8082 --id=localhost:8082 --peers=localhost:8080

## API Endpoints

- `GET /count` - Get the current counter value
- `POST /count/increment` - Increment the counter

## Testing

### Manual Testing

1. Start multiple nodes as described above

2. Test counter operations using curl:
```bash
# Get counter value
curl http://localhost:8080/count

# Increment counter
curl -X POST http://localhost:8080/count/increment
curl -X POST http://localhost:8081/count/increment
```

3. Verify that the counter values are synchronized across all nodes:
```bash
# Check counter value on different nodes
curl http://localhost:8081/count
curl http://localhost:8082/count
```


## Features

- Distributed counter implementation
- Node discovery and synchronization
- REST API for counter operations
- Eventual consistency across nodes