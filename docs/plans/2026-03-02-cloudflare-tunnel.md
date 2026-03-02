# Cloudflare Tunnel + Uptime Format Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add seconds to uptime display (`d:hh:mm:ss`) and start an optional Cloudflare temporary tunnel on launch, printing a ready-to-paste forum embed code when the URL is assigned.

**Architecture:** `formatUptime` is exported and updated to include seconds with zero-padded colons. In `main.go`, a `startTunnel` helper optionally spawns `cloudflared tunnel --url http://localhost:8080` as a child process, scans its stderr for the `trycloudflare.com` URL, and prints the `[img]` embed code. The Dockerfile copies the `cloudflared` binary from the official `cloudflare/cloudflared` image via multi-stage build.

**Tech Stack:** Go stdlib (`os/exec`, `bufio`, `strings`), cloudflare/cloudflared Docker image

---

## Task 1: Fix uptime format to `d:hh:mm:ss`

**Files:**
- Modify: `internal/stats/collector.go:129-134`
- Modify: `internal/stats/collector_test.go`

**Step 1: Write the failing test**

Add to `internal/stats/collector_test.go`:
```go
func TestFormatUptime(t *testing.T) {
	cases := []struct {
		seconds  uint64
		expected string
	}{
		{310965, "3:14:22:45"}, // 3d 14h 22m 45s
		{7509, "0:02:05:09"},   // 0d 2h 5m 9s
		{0, "0:00:00:00"},      // zero
		{86400, "1:00:00:00"},  // exactly 1 day
	}
	for _, tc := range cases {
		got := stats.FormatUptime(tc.seconds)
		if got != tc.expected {
			t.Errorf("FormatUptime(%d) = %q, want %q", tc.seconds, got, tc.expected)
		}
	}
}
```

**Step 2: Run test to verify it fails**
```bash
go test ./internal/stats/... -run TestFormatUptime -v
```
Expected: FAIL — `stats.FormatUptime` undefined (unexported currently)

**Step 3: Update `formatUptime` in `internal/stats/collector.go`**

Replace lines 129-134:
```go
// FormatUptime formats total seconds as d:hh:mm:ss.
func FormatUptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return fmt.Sprintf("%d:%02d:%02d:%02d", days, hours, minutes, secs)
}
```

Then update the call site on line 112:
```go
uptimeStr := FormatUptime(uptime)
```

**Step 4: Run test to verify it passes**
```bash
go test ./internal/stats/... -v
```
Expected: ALL PASS (including existing tests)

**Step 5: Update the existing uptime format assertion in `TestCollectorReturnsValidStats`**

The existing test only checks `s.UptimeStr == ""` — no format assertion to update there. But verify the real collector now returns colon-format:
```bash
go test ./internal/stats/... -run TestCollectorReturnsValidStats -v
```
Expected: PASS

**Step 6: Commit**
```bash
git add internal/stats/
git commit -m "feat: uptime format d:hh:mm:ss"
```

---

## Task 2: Add optional Cloudflare tunnel to main.go

**Files:**
- Modify: `main.go`

**Step 1: Add `startTunnel` function to `main.go`**

Add the following imports to `main.go`:
```go
import (
	"bufio"
	"context"
	"fmt"
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
```

Add these two functions after `main()`:
```go
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

func scanTunnelURL(ctx context.Context, r io.Reader) {
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
```

Also add `"io"` to the imports (for `io.Reader`).

**Step 2: Call `startTunnel` in `main()` after the HTTP server goroutine**

In `main()`, insert `startTunnel(ctx)` just before `<-ctx.Done()`:
```go
	go func() {
		log.Println("listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	startTunnel(ctx)  // ← add this line

	// Wait for shutdown signal
	<-ctx.Done()
```

**Step 3: Build and verify it compiles**
```bash
go build ./...
```
Expected: no errors

**Step 4: Smoke test without cloudflared (should warn and continue)**
```bash
go build -o live-sys-stats . && PATH="" ./live-sys-stats &
sleep 2
curl -s -I http://localhost:8080/sig.png | head -3
kill %1
rm live-sys-stats
```
Expected output includes:
```
cloudflared not found in PATH — running without tunnel
```
And curl returns `HTTP/1.1 200 OK`.

**Step 5: Commit**
```bash
git add main.go
git commit -m "feat: optional cloudflare tunnel with forum embed output"
```

---

## Task 3: Install cloudflared in the Docker image

**Files:**
- Modify: `Dockerfile`

**Step 1: Update Dockerfile to copy cloudflared from official image**

Replace the full `Dockerfile` with:
```dockerfile
# Get cloudflared binary
FROM cloudflare/cloudflared:latest AS cloudflared

# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o live-sys-stats .

# Runtime stage
FROM alpine:3.19
WORKDIR /app
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /app/live-sys-stats .
COPY --from=cloudflared /usr/local/bin/cloudflared /usr/local/bin/cloudflared
EXPOSE 8080
USER appuser
ENTRYPOINT ["./live-sys-stats"]
```

**Step 2: Build the Docker image**
```bash
docker compose build
```
Expected: build succeeds

**Step 3: Verify cloudflared is present in the image**
```bash
docker run --rm --entrypoint cloudflared live_sys_stat_via_image-live-sys-stats version
```
Expected: prints cloudflared version string

**Step 4: Verify image size is still reasonable**
```bash
docker images | grep live-sys-stats
```
Expected: under 60MB (cloudflared binary is ~30MB)

**Step 5: Run container and check tunnel starts**
```bash
docker compose up
```
Expected log output (after a few seconds):
```
cloudflared tunnel starting...
════════════════════════════════════════
SYSTEM ONLINE! PASTE THIS INTO YOUR FORUM:
[img]https://xxxx.trycloudflare.com/sig.png[/img]
════════════════════════════════════════
```

**Step 6: Stop and commit**
```bash
docker compose down
git add Dockerfile
git commit -m "feat: install cloudflared in docker image via multi-stage copy"
```

---

## Quick Reference

| Command | Purpose |
|---|---|
| `go test ./... -v` | Run all tests |
| `docker compose up` | Run with tunnel |
| `docker compose up 2>&1 \| grep trycloudflare` | Extract tunnel URL from logs |
