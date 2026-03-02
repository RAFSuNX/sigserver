package main

import (
	"log"
	"net/http"
	"time"

	"live-sys-stats/internal/renderer"
	"live-sys-stats/internal/server"
	"live-sys-stats/internal/stats"
)

func main() {
	collector := stats.NewCollector()
	broker := server.NewBroker()
	handler := server.NewHandler(broker)

	// Warm up network baseline (first collect has no rate data)
	if _, err := collector.Collect(); err != nil {
		log.Printf("warn: warm-up collect failed: %v", err)
	}

	// Publish a new frame every second
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			s, err := collector.Collect()
			if err != nil {
				log.Printf("collect error: %v", err)
				continue
			}
			frame, err := renderer.Render(*s)
			if err != nil {
				log.Printf("render error: %v", err)
				continue
			}
			broker.Publish(frame)
		}
	}()

	http.Handle("/sig.png", handler)
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
