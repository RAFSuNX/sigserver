package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"live-sys-stats/internal/renderer"
	"live-sys-stats/internal/server"
	"live-sys-stats/internal/stats"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	collector := stats.NewCollector()
	broker := server.NewBroker()
	handler := server.NewHandler(broker)

	// Warm up network baseline (first collect has no rate data)
	if _, err := collector.Collect(); err != nil {
		log.Printf("warn: warm-up collect failed: %v", err)
	}

	// Publish a new frame every second until context is cancelled
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s, err := collector.Collect()
				if err != nil {
					log.Printf("collect error: %v", err)
					continue
				}
				// renderer.Render returns a fresh []byte each call,
				// so the slice is never mutated after Publish.
				frame, err := renderer.Render(*s)
				if err != nil {
					log.Printf("render error: %v", err)
					continue
				}
				broker.Publish(frame)
			}
		}
	}()

	srv := &http.Server{Addr: ":8080"}
	http.Handle("/sig.png", handler)

	go func() {
		log.Println("listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
