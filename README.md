# ClusterGuard

Real time fault detection system for distributed operational environments.
Consumes event streams from Apache Kafka, detects anomalies using a sliding-window
statistical model, triggers automated resolution workflows and exposes Prometheus
metrics for observability.

Built to explore end to end ownership of a Kubernetes native service from manifest
authoring and Helm packaging through to debugging pod restarts with `kubectl` and
writing a runbook for on call response.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Kubernetes Cluster                        │
│                                                                  │
│  ┌──────────┐    ┌─────────────────┐    ┌──────────────────┐   │
│  │  Kafka   │───▶│  ClusterGuard   │───▶│  Alert Manager   │   │
│  │ (source) │    │   (consumer)    │    │  (webhook sink)  │   │
│  └──────────┘    │                 │    └──────────────────┘   │
│                  │ /metrics ──────────▶ Prometheus             │
│                  │ /healthz           └──────▶ Grafana          │
│                  └─────────────────┘                            │
│                         │                                        │
│                    ConfigMap (thresholds)                        │
└─────────────────────────────────────────────────────────────────┘
```

**Data flow**

1. Events arrive on the `ops.events` Kafka topic (JSON, ~2M/day in load tests)
2. Consumer goroutines process partitions concurrently
3. Each event is scored against a sliding window anomaly detector
4. Anomalies above threshold emit a Prometheus counter and POST to a configurable webhook
5. All decisions are written to stdout as structured JSON logs

---

## Project structure

```
ClusterGuard/
├── cmd/
│ └── main.go  # entrypoint, flag parsing, signal handling
│        
├── internal/
│   ├── detector/
│   │   ├── anomaly.go       # sliding-window z-score detector
│   │   └── anomaly_test.go
│   ├── metrics/
│   │   └── prometheus.go    # Prometheus counters, histograms, gauges
│   └── webhook/
│       └── alert.go         # HTTP POST to configurable alert endpoint
├── deploy/
│   ├── k8s/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   
│   └── helm/
│       ├── Chart.yaml
│       ├── values.yaml
│        
├── .github/
│   └── workflows/
│       ├── ci.yaml
├── docs/
│   └── runbook.md
├── go.mod
└── README.md
```

---

## Getting started

### Prerequisites

- Go 1.22+
- Docker
- A running Kafka broker (local: `docker compose up kafka`)
- `kubectl` configured against a cluster (local: `kind create cluster`)

### Run locally

```bash
git clone https://github.com/RiyaJ6/ClusterGuard
cd ClusterGuard

# start a local kafka
docker run -d --name kafka -p 9092:9092 \
  -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://localhost:9092 \
  -e KAFKA_LISTENERS=PLAINTEXT://0.0.0.0:9092 \
  -e KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181 \
  confluentinc/cp-kafka:latest

# build and run
go build -o ClusterGuard ./cmd/ClusterGuard
./ClusterGuard \
  --brokers localhost:9092 \
  --topic ops.events \
  --group ClusterGuard \
  --webhook-url http://localhost:8080/alerts \
  --metrics-port 9090
```

Metrics will be available at `http://localhost:9090/metrics`.

### Run tests

```bash
go test ./... -v -race
```

### Deploy to Kubernetes

```bash
# raw manifests
kubectl apply -f deploy/k8s/

# or via Helm
helm install ClusterGuard deploy/helm/ClusterGuard \
  --set kafka.brokers=kafka:9092 \
  --set webhook.url=http://alertmanager:9093/webhook
```

---

## Configuration

All configuration is read from environment variables (set via ConfigMap in Kubernetes).

| Variable | Default | Description |
|---|---|---|
| `KAFKA_BROKERS` | `localhost:9092` | Comma separated broker list |
| `KAFKA_TOPIC` | `ops.events` | Topic to consume |
| `KAFKA_GROUP` | `clusterguard` | Consumer group ID |
| `WINDOW_SIZE` | `100` | Sliding window size for anomaly detection |
| `ZSCORE_THRESHOLD` | `3.0` | Z-score threshold for anomaly flag |
| `WEBHOOK_URL` | `` | Alert webhook endpoint |
| `METRICS_PORT` | `9090` | Prometheus metrics port |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |

---

## Observability

**Metrics exposed at `/metrics`**

| Metric | Type | Description |
|---|---|---|
| `clusterguard_events_processed_total` | Counter | Total events consumed |
| `clusterguard_anomalies_detected_total` | Counter | Anomalies above threshold |
| `clusterguard_processing_duration_seconds` | Histogram | Per-event processing time |
| `clusterguard_consumer_lag` | Gauge | Kafka consumer lag by partition |
| `clusterguard_webhook_errors_total` | Counter | Failed alert deliveries |

**Structured log fields** (JSON to stdout)

```json
{
  "level": "info",
  "ts": "2025-03-14T09:26:00Z",
  "event_id": "evt_8f3a",
  "partition": 2,
  "offset": 18432,
  "score": 4.21,
  "anomaly": true,
  "processing_ms": 1.4,
  "msg": "anomaly detected"
}
```

See [`docs/runbook.md`](docs/runbook.md) for dashboard setup and on call response steps.

---

## Debugging pod restarts — What I learned

During development I hit three real restart loops that were instructive to trace:

**OOMKill** — the consumer was buffering all in flight events in memory before processing.
With a high throughput topic this blew the 128Mi memory limit within minutes.
Fix: switched to a bounded channel with backpressure tuned limit to 256Mi after
measuring actual RSS under load with `kubectl top pod`.

**Failed readiness probe** — the `/healthz` endpoint returned 200 only after the Kafka
consumer group had rebalanced which takes 3 to 10 seconds on startup. The readiness probe
was firing at 2 seconds. Fix: added an `initialDelaySeconds: 15` to the probe.

**Goroutine race** — the anomaly detector's sliding window was a shared slice accessed
by multiple partition goroutines without a lock. Found with `go test -race`.
Fix: one detector instance per partition goroutine no shared state.

Each fix was validated by deploying to a local `kind` cluster and watching
`kubectl get pods -w` until the restart count held at 0 for 5 minutes.

---

## Terraform

The `terraform/` directory provisions the AWS infrastructure used in staging:
VPC, EKS node group, IAM role for the service account (IRSA), and an S3 bucket
for state. See [`terraform/README.md`](terraform/README.md) for usage.

---

<div align="center">
  
  **Self healing across a cluster of servers**

