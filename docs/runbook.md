# ClusterGuard Runbook

This document covers dashboard setup, alert definitions and on-call response steps.
It is the reference point for anyone paged about a ClusterGuard alert.

---

## Grafana dashboard

Import `docs/grafana-dashboard.json` into your Grafana instance.
The dashboard has four panels:

| Panel | What it shows |
|---|---|
| Events / sec | Ingestion rate — should track Kafka producer throughput |
| Anomaly rate | Anomalies per minute — baseline is near zero in steady state |
| Processing p99 | 99th percentile event processing latency — alert if > 50ms |
| Consumer lag | Per-partition lag — sustained lag means the consumer is behind |

---

## Alerts

### `ClusterGuardAnomalyRateHigh`

**Condition:** `rate(clusterguard_anomalies_detected_total[5m]) > 10`
**Severity:** warning
**Meaning:** More than 10 anomalies per second in the last 5 minutes.
This can be a genuine signal spike or a detector misconfiguration.

**Steps:**
1. Check the structured logs: `kubectl logs -l app=clusterguard --since=10m | jq 'select(.anomaly==true)'`
2. Look at the `score` and `z_score` fields — are these plausibly anomalous events or noise?
3. If noise: increase `ZSCORE_THRESHOLD` in the ConfigMap and roll the deployment.
4. If genuine: check the upstream data source for an incident.

---

### `ClusterGuardConsumerLagHigh`

**Condition:** `clusterguard_consumer_lag > 50000`
**Severity:** warning
**Meaning:** The consumer is significantly behind the producer.

**Steps:**
1. `kubectl top pod -l app=clusterguard` — check CPU and memory.
2. If OOMKilled: `kubectl describe pod <pod>` — check `Last State` for OOMKill reason.
   Increase memory limit in `values.yaml` and redeploy.
3. If CPU throttled: increase CPU limit or add a replica.
4. Check Kafka broker health — lag can also be caused by broker-side issues.

---

### `ClusterGuardWebhookErrorsHigh`

**Condition:** `rate(clusterguard_webhook_errors_total[5m]) > 1`
**Severity:** warning
**Meaning:** Alert deliveries are failing.

**Steps:**
1. Check if the webhook endpoint is reachable from the cluster.
2. `kubectl exec -it <pod> -- wget -qO- http://alertmanager:9093/webhook` — basic reachability test.
3. If endpoint is down: alerts will queue up and deliver once it recovers.
   ClusterGuard does not buffer — anomalies detected during downtime will not be re-sent.

---

## Debugging pod restarts

If you see `kubectl get pods -l app=clusterguard` showing a non-zero RESTARTS count:

```bash
# see why the pod last terminated
kubectl describe pod <pod-name> | grep -A 5 "Last State"

# read logs from the previous (crashed) container
kubectl logs <pod-name> --previous
```

**Common causes seen during development:**

- **OOMKill**: memory limit too low for current throughput.
  Fix: raise `resources.limits.memory` in `values.yaml`.

- **Readiness probe failure on startup**: Kafka rebalance takes longer than `initialDelaySeconds`.
  Fix: increase `readinessProbe.initialDelaySeconds`.

- **Goroutine race on shared state**: run `go test -race ./...` locally to reproduce.

---

## Escalation path

1. On-call engineer attempts the steps above (15 min time-box).
2. If unresolved: page the platform team lead.
3. If Kafka-related: page the data infrastructure team.
4. All incidents logged in the incident tracker with timeline, root cause, and fix.
