# gRPC Deduplication Test Application

This repository contains a gRPC Go application designed to validate and verify static deduplication edge cases for Keploy. It exercises various gRPC wire formats (different scalar values, optional fields, repeated fields, maps, oneofs, and error statuses) to ensure deduplication algorithms correctly group identical structures while preserving distinct schemas.

---

## Repository Structure

* **[proto/dedup.proto](file:///home/shivani/grpc-dedup-test/proto/dedup.proto)**: Contains the Protobuf definition for `DedupService` and the schemas of all test groups.
* **[main.go](file:///home/shivani/grpc-dedup-test/main.go)**: The server implementation in Go, exposing the gRPC endpoints on port `:50051` with reflection enabled.
* **[scripts/generate-traffic.sh](file:///home/shivani/grpc-dedup-test/scripts/generate-traffic.sh)**: A test harness script that drives 28 gRPC calls using `grpcurl` to test the deduplication capabilities.
* **[Dockerfile](file:///home/shivani/grpc-dedup-test/Dockerfile)**: Dockerfile to build a lightweight alpine container containing the server binary.
* **[k8s/deployment.yaml](file:///home/shivani/grpc-dedup-test/k8s/deployment.yaml)**: Kubernetes manifests for deploying the application inside the `dedup-test` namespace.

---

## Test Groups & Deduplication Logic

The application exercises 6 different structural patterns, sending a total of **28 calls** which should be deduplicated down to exactly **11 unique test cases**:

| Group | Method | Description | Total Calls | Expected Unique TCs |
| :--- | :--- | :--- | :---: | :---: |
| **A** | `GetItem` | Same structure, varying scalar values. | 5 | **1** |
| **B** | `GetWidget` | Optional nested field present vs. absent on wire. | 4 | **2** |
| **C** | `RiskyCall` | Returns success (grpc-status=0) vs. error (grpc-status=3). | 4 | **2** |
| **D** | `GetReport` | Deeply nested message with maps and lists. | 3 | **1** |
| **E** | `GetVariant` | `oneof` payload testing three separate wire field types (string, int32, bool). | 6 | **3** |
| **F** | `ListItems` | Non-empty vs. empty repeated field (field absent on wire). | 4 | **2** |
| **Total** | | | **28** | **11** |

---

## How to Get Started

### Prerequisites

* Go 1.25.0+ installed
* `grpcurl` installed (for running the traffic generator script)

### 1. Run Locally

1. **Build the server**:
   ```bash
   go build -o build/server main.go
   ```

2. **Start the server**:
   ```bash
   ./build/server
   ```
   The server will start listening for gRPC requests on port `50051`.

3. **Generate Test Traffic**:
   While the server is running, execute the traffic generator script to send the 28 test calls:
   ```bash
   ./scripts/generate-traffic.sh
   ```

---

## Running with Docker & Kubernetes

### Docker

1. **Cross-compile the Go binary** (target OS: Linux):
   ```bash
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/server main.go
   ```

2. **Build the container image**:
   ```bash
   docker build -t grpc-dedup-test:latest .
   ```

3. **Run the container**:
   ```bash
   docker run -p 50051:50051 grpc-dedup-test:latest
   ```

### Kubernetes

The repository includes a ready-to-use manifest to deploy the app:

```bash
kubectl apply -f k8s/deployment.yaml
```
This deploys the service in the `dedup-test` namespace exposing the gRPC server on port `50051`.
