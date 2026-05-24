package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RiyaJ6/ClusterGuard/internal/detector"
	"github.com/RiyaJ6/ClusterGuard/internal/metrics"
	"github.com/RiyaJ6/ClusterGuard/internal/webhook"
	"github.com/prometheus/client_golang/prometheus/promhttp"

)

func main() {
	brokers := flag.String("brokers", envOr("KAFKA_BROKERS", "localhost:9092"), "Kafka broker list")
	topic := flag.String("topic", envOr("KAFKA_TOPIC", "ops.events"), "Kafka topic")
	group := flag.String("group", envOr("KAFKA_GROUP", "ClusterGuard"), "Consumer group ID")
	windowSize := flag.Int("window-size", 100, "Sliding window size for anomaly detection")
	threshold := flag.Float64("threshold", 3.0, "Z-score threshold")
	webhookURL := flag.String("webhook-url", envOr("WEBHOOK_URL", ""), "Alert webhook endpoint")
	metricsPort := flag.String("metrics-port", envOr("METRICS_PORT", "9090"), "Prometheus metrics port")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	m := metrics.New()
	newDetector := func() *detector.SlidingWindow {
		return detector.New(*windowSize, *threshold)
	}
	alerter := webhook.New(*webhookURL, logger)

	// Suppress unused variable warnings for active components
	_, _, _ = m, newDetector, alerter

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	srv := &http.Server{
		Addr:         ":" + *metricsPort,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("metrics server listening", "port", *metricsPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", "err", err)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	logger.Info("clusterguard starting", "brokers", *brokers, "topic", *topic, "group", *group)

	// Block here until the application receives a shutdown signal
	<-ctx.Done()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)

	logger.Info("clusterguard stopped cleanly")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
