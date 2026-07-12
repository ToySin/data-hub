package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	broker := flag.String("broker", "tcp://localhost:1883", "MQTT broker URL")
	clientID := flag.String("client-id", "data-hub", "MQTT client ID")
	flag.Parse()

	// ctx is cancelled on SIGINT/SIGTERM, driving graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := NewMQTTClient(*broker, *clientID)
	if err != nil {
		slog.Error("failed to connect to mqtt broker", "error", err)
		os.Exit(1)
	}

	cache := &StateCache{}
	// Buffered so a transient publisher slowdown does not immediately block the
	// MQTT subscribe callbacks (which feed loss-critical navigation events).
	eventCh := make(chan PublishEvent, 256)

	aggregator := NewStateAggregator(client, cache, eventCh)
	publisher := NewMessagePublisher(client, eventCh, cache)

	if err := publisher.Start(ctx); err != nil {
		slog.Error("failed to start publisher", "error", err)
		os.Exit(1)
	}
	if err := aggregator.Start(ctx); err != nil {
		slog.Error("failed to start aggregator", "error", err)
		os.Exit(1)
	}

	slog.Info("data-hub started", "broker", *broker, "client_id", *clientID)

	<-ctx.Done()
	slog.Info("shutting down")

	// Order matters: ctx is already cancelled (our goroutines are winding down),
	// so disconnect the broker afterwards to flush and close cleanly.
	client.Disconnect()
	slog.Info("stopped")
}
