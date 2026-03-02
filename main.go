package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"os/signal"
	"strings"
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

	startTunnel(ctx)

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

// startTunnel optionally starts cloudflared and prints the forum embed code.
// If cloudflared is not in PATH it logs a warning and returns immediately.
func startTunnel(ctx context.Context) {
	if _, err := exec.LookPath("cloudflared"); err != nil {
		log.Println("cloudflared not found in PATH — running without tunnel")
		return
	}

	cmd := exec.CommandContext(ctx, "cloudflared", "tunnel", "--url", "http://localhost:8080")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("tunnel pipe: %v", err)
		return
	}
	if err := cmd.Start(); err != nil {
		log.Printf("tunnel start: %v", err)
		return
	}
	log.Println("cloudflared tunnel starting...")

	go scanTunnelURL(ctx, stderr)

	go func() {
		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			log.Printf("tunnel exited: %v", err)
		}
	}()
}

func scanTunnelURL(_ context.Context, r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "trycloudflare.com") {
			continue
		}
		for _, field := range strings.Fields(line) {
			field = strings.Trim(field, "|")
			if strings.HasPrefix(field, "https://") && strings.Contains(field, "trycloudflare.com") {
				printEmbedCode(field + "/sig.png")
				return
			}
		}
	}
}

func printEmbedCode(url string) {
	fmt.Println("════════════════════════════════════════")
	fmt.Println("SYSTEM ONLINE! PASTE THIS INTO YOUR FORUM:")
	fmt.Printf("[img]%s[/img]\n", url)
	fmt.Println("════════════════════════════════════════")
}
