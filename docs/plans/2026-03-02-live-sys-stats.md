# Live System Stats Image — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go service that streams live system stats as a continuously-updating PNG image via `multipart/x-mixed-replace` HTTP — embeddable in forums with a plain `<img>` tag.

**Architecture:** A stats collector reads CPU, RAM, disk, network, and uptime every second using gopsutil, tracking network byte deltas for rate calculation. An image renderer draws stats onto a PNG using golang.org/x/image basicfont with a Claude Code dark theme. An HTTP server broadcasts each frame to all connected clients using a pub/sub broker pattern.

**Tech Stack:** Go 1.22, `github.com/shirou/gopsutil/v3`, `golang.org/x/image` (basicfont + font drawer), Docker multi-stage build (alpine runtime)

---

## Task 1: Initialize project structure

**Files:**
- Create: `go.mod`
- Create: `main.go` (empty stub)
- Create: `internal/stats/collector.go` (empty stub)
- Create: `internal/renderer/image.go` (empty stub)
- Create: `internal/server/broker.go` (empty stub)

**Step 1: Create directory structure**

```bash
mkdir -p internal/stats internal/renderer internal/server
```

**Step 2: Initialize Go module**

```bash
go mod init live-sys-stats
```

**Step 3: Add dependencies**

```bash
go get github.com/shirou/gopsutil/v3@latest
go get golang.org/x/image@latest
```

**Step 4: Create empty stubs**

`main.go`:
```go
package main

func main() {}
```

`internal/stats/collector.go`:
```go
package stats
```

`internal/renderer/image.go`:
```go
package renderer
```

`internal/server/broker.go`:
```go
package server
```

**Step 5: Verify it compiles**

```bash
go build ./...
```
Expected: no output (success)

**Step 6: Commit**

```bash
git init
git add .
git commit -m "feat: initialize go module and project structure"
```

---

## Task 2: Stats data structure and collector

**Files:**
- Modify: `internal/stats/collector.go`
- Create: `internal/stats/collector_test.go`

**Step 1: Write the failing test**

`internal/stats/collector_test.go`:
```go
package stats_test

import (
	"testing"
	"live-sys-stats/internal/stats"
)

func TestCollectorReturnsValidStats(t *testing.T) {
	c := stats.NewCollector()

	s, err := c.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.Hostname == "" {
		t.Error("hostname should not be empty")
	}
	if s.CPUPercent < 0 || s.CPUPercent > 100 {
		t.Errorf("cpu percent out of range: %f", s.CPUPercent)
	}
	if s.RAMTotalGB <= 0 {
		t.Errorf("ram total should be positive, got %f", s.RAMTotalGB)
	}
	if s.DiskTotalGB <= 0 {
		t.Errorf("disk total should be positive, got %f", s.DiskTotalGB)
	}
	if s.UptimeStr == "" {
		t.Error("uptime string should not be empty")
	}
}

func TestNetworkRateOnSecondCall(t *testing.T) {
	c := stats.NewCollector()
	c.Collect() // prime the previous sample

	s, err := c.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// rates should be non-negative after second call
	if s.NetUpMBps < 0 {
		t.Errorf("net up rate should be >= 0, got %f", s.NetUpMBps)
	}
	if s.NetDownMBps < 0 {
		t.Errorf("net down rate should be >= 0, got %f", s.NetDownMBps)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/stats/... -v
```
Expected: FAIL — `stats.NewCollector` undefined

**Step 3: Implement the collector**

`internal/stats/collector.go`:
```go
package stats

import (
	"fmt"
	"os"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"
)

// Stats holds a single snapshot of all system metrics.
type Stats struct {
	Hostname    string
	CPUPercent  float64
	CPUFreqGHz  float64
	LoadAvg     [3]float64
	RAMUsedGB   float64
	RAMTotalGB  float64
	DiskUsedGB  float64
	DiskTotalGB float64
	NetUpMBps   float64
	NetDownMBps float64
	UptimeStr   string
}

// Collector reads system stats, tracking previous network counters for rate calc.
type Collector struct {
	prevNetBytes [2]uint64 // [sent, recv]
	prevNetTime  time.Time
}

// NewCollector creates a Collector and primes the network baseline.
func NewCollector() *Collector {
	return &Collector{}
}

// Collect reads all system stats and returns a Stats snapshot.
func (c *Collector) Collect() (*Stats, error) {
	hostname, _ := os.Hostname()

	// CPU percent (100ms interval for accuracy)
	percents, err := cpu.Percent(100*time.Millisecond, false)
	if err != nil {
		return nil, fmt.Errorf("cpu percent: %w", err)
	}
	cpuPct := 0.0
	if len(percents) > 0 {
		cpuPct = percents[0]
	}

	// CPU frequency
	freqs, _ := cpu.Info()
	cpuFreqGHz := 0.0
	if len(freqs) > 0 {
		cpuFreqGHz = freqs[0].Mhz / 1000.0
	}

	// Load average
	avg, err := load.Avg()
	if err != nil {
		return nil, fmt.Errorf("load avg: %w", err)
	}

	// Memory
	vmem, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("memory: %w", err)
	}

	// Disk (root partition)
	usage, err := disk.Usage("/")
	if err != nil {
		return nil, fmt.Errorf("disk: %w", err)
	}

	// Network rate
	netCounters, err := psnet.IOCounters(false)
	netUpMBps, netDownMBps := 0.0, 0.0
	now := time.Now()
	if err == nil && len(netCounters) > 0 {
		sent := netCounters[0].BytesSent
		recv := netCounters[0].BytesRecv
		if !c.prevNetTime.IsZero() {
			elapsed := now.Sub(c.prevNetTime).Seconds()
			if elapsed > 0 {
				netUpMBps = float64(sent-c.prevNetBytes[0]) / elapsed / 1024 / 1024
				netDownMBps = float64(recv-c.prevNetBytes[1]) / elapsed / 1024 / 1024
			}
		}
		c.prevNetBytes = [2]uint64{sent, recv}
		c.prevNetTime = now
	}

	// Uptime
	uptime, err := host.Uptime()
	if err != nil {
		return nil, fmt.Errorf("uptime: %w", err)
	}
	uptimeStr := formatUptime(uptime)

	return &Stats{
		Hostname:    hostname,
		CPUPercent:  cpuPct,
		CPUFreqGHz:  cpuFreqGHz,
		LoadAvg:     [3]float64{avg.Load1, avg.Load5, avg.Load15},
		RAMUsedGB:   float64(vmem.Used) / 1024 / 1024 / 1024,
		RAMTotalGB:  float64(vmem.Total) / 1024 / 1024 / 1024,
		DiskUsedGB:  float64(usage.Used) / 1024 / 1024 / 1024,
		DiskTotalGB: float64(usage.Total) / 1024 / 1024 / 1024,
		NetUpMBps:   netUpMBps,
		NetDownMBps: netDownMBps,
		UptimeStr:   uptimeStr,
	}, nil
}

func formatUptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/stats/... -v
```
Expected: PASS for both tests

**Step 5: Commit**

```bash
git add internal/stats/
git commit -m "feat: add stats collector with network rate calculation"
```

---

## Task 3: Image renderer

**Files:**
- Modify: `internal/renderer/image.go`
- Create: `internal/renderer/image_test.go`

**Step 1: Write the failing test**

`internal/renderer/image_test.go`:
```go
package renderer_test

import (
	"bytes"
	"image"
	_ "image/png"
	"testing"

	"live-sys-stats/internal/renderer"
	"live-sys-stats/internal/stats"
)

func TestRenderReturnsPNG(t *testing.T) {
	s := &stats.Stats{
		Hostname:    "testbox",
		CPUPercent:  42.5,
		CPUFreqGHz:  3.6,
		LoadAvg:     [3]float64{0.8, 1.2, 0.9},
		RAMUsedGB:   6.2,
		RAMTotalGB:  16.0,
		DiskUsedGB:  120.3,
		DiskTotalGB: 500.0,
		NetUpMBps:   1.2,
		NetDownMBps: 4.5,
		UptimeStr:   "3d 14h 22m",
	}

	data, err := renderer.Render(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("render returned empty bytes")
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode image: %v", err)
	}
	if format != "png" {
		t.Errorf("expected png, got %s", format)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 600 || bounds.Dy() != 160 {
		t.Errorf("expected 600x160, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/renderer/... -v
```
Expected: FAIL — `renderer.Render` undefined

**Step 3: Implement the renderer**

`internal/renderer/image.go`:
```go
package renderer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"live-sys-stats/internal/stats"
)

const (
	imgW = 600
	imgH = 160
)

var (
	bgColor     = color.RGBA{R: 26, G: 26, B: 26, A: 255}   // #1a1a1a
	textColor   = color.RGBA{R: 224, G: 224, B: 224, A: 255} // #e0e0e0
	accentColor = color.RGBA{R: 217, G: 119, B: 6, A: 255}   // #d97706
	dimColor    = color.RGBA{R: 100, G: 100, B: 100, A: 255} // dim separator
)

// Render draws a stats snapshot onto a 600x160 PNG and returns the bytes.
func Render(s *stats.Stats) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	draw.Draw(img, img.Bounds(), image.NewUniform(bgColor), image.Point{}, draw.Src)

	// Hostname header in orange
	drawText(img, s.Hostname, 10, 20, accentColor)

	// Separator line
	drawHLine(img, 10, imgW-10, 28, dimColor)

	// Stats rows (y positions: 48, 68, 88, 108, 128)
	drawText(img, fmt.Sprintf("CPU   %5.1f%%  @  %.2f GHz    load: %.2f  %.2f  %.2f",
		s.CPUPercent, s.CPUFreqGHz, s.LoadAvg[0], s.LoadAvg[1], s.LoadAvg[2]),
		10, 48, textColor)

	drawText(img, fmt.Sprintf("RAM   %.1f / %.1f GB",
		s.RAMUsedGB, s.RAMTotalGB),
		10, 68, textColor)

	drawText(img, fmt.Sprintf("DISK  %.1f / %.1f GB",
		s.DiskUsedGB, s.DiskTotalGB),
		10, 88, textColor)

	drawText(img, fmt.Sprintf("NET   up %.2f MB/s    down %.2f MB/s",
		s.NetUpMBps, s.NetDownMBps),
		10, 108, textColor)

	drawText(img, fmt.Sprintf("UP    %s", s.UptimeStr),
		10, 128, textColor)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawText(img *image.RGBA, text string, x, y int, clr color.Color) {
	d := &xfont.Drawer{
		Dst:  img,
		Src:  image.NewUniform(clr),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(text)
}

func drawHLine(img *image.RGBA, x0, x1, y int, clr color.Color) {
	for x := x0; x <= x1; x++ {
		img.Set(x, y, clr)
	}
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/renderer/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/renderer/
git commit -m "feat: add PNG renderer with claude code dark theme"
```

---

## Task 4: HTTP multipart streaming server

**Files:**
- Modify: `internal/server/broker.go`
- Create: `internal/server/handler.go`
- Create: `internal/server/handler_test.go`

**Step 1: Write the failing test**

`internal/server/handler_test.go`:
```go
package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"live-sys-stats/internal/server"
)

func TestHandlerContentType(t *testing.T) {
	b := server.NewBroker()
	h := server.NewHandler(b)

	req := httptest.NewRequest(http.MethodGet, "/sig.png", nil)
	rec := httptest.NewRecorder()

	// Run handler in goroutine since it streams indefinitely
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()

	// Give it a moment to write headers
	time.Sleep(50 * time.Millisecond)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "multipart/x-mixed-replace") {
		t.Errorf("expected multipart content-type, got: %s", ct)
	}
}

func TestBrokerPublishReachesSubscriber(t *testing.T) {
	b := server.NewBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	frame := []byte("test-frame")
	b.Publish(frame)

	select {
	case got := <-ch:
		if string(got) != string(frame) {
			t.Errorf("expected %q, got %q", frame, got)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for published frame")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/server/... -v
```
Expected: FAIL — `server.NewBroker` undefined

**Step 3: Implement the broker**

`internal/server/broker.go`:
```go
package server

import "sync"

// Broker distributes PNG frames to all connected HTTP clients.
type Broker struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[chan []byte]struct{}),
	}
}

// Subscribe returns a channel that receives each published frame.
func (b *Broker) Subscribe() chan []byte {
	ch := make(chan []byte, 1)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a client channel.
func (b *Broker) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

// Publish sends a frame to all subscribers, dropping slow clients.
func (b *Broker) Publish(frame []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- frame:
		default: // drop frame for slow/unresponsive clients
		}
	}
}
```

**Step 4: Implement the HTTP handler**

`internal/server/handler.go`:
```go
package server

import (
	"fmt"
	"net/http"
)

const boundary = "frame"

// Handler streams PNG frames as multipart/x-mixed-replace.
type Handler struct {
	broker *Broker
}

func NewHandler(b *Broker) *Handler {
	return &Handler{broker: b}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := h.broker.Subscribe()
	defer h.broker.Unsubscribe(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-ch:
			fmt.Fprintf(w, "--%s\r\nContent-Type: image/png\r\nContent-Length: %d\r\n\r\n", boundary, len(frame))
			w.Write(frame)
			fmt.Fprintf(w, "\r\n")
			flusher.Flush()
		}
	}
}
```

**Step 5: Run test to verify it passes**

```bash
go test ./internal/server/... -v
```
Expected: PASS for both tests

**Step 6: Commit**

```bash
git add internal/server/
git commit -m "feat: add multipart streaming broker and HTTP handler"
```

---

## Task 5: Wire main.go

**Files:**
- Modify: `main.go`

**Step 1: Implement main.go**

```go
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
	collector.Collect()

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
			frame, err := renderer.Render(s)
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
```

**Step 2: Build and run locally**

```bash
go build -o live-sys-stats .
./live-sys-stats
```
Expected output: `listening on :8080`

**Step 3: Smoke test in browser**

Open `http://localhost:8080/sig.png` — you should see the stats image updating every second without page refresh.

**Step 4: Stop the server and commit**

```bash
git add main.go
git commit -m "feat: wire main loop with 1s ticker and HTTP server"
```

---

## Task 6: Dockerfile and docker-compose

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`

**Step 1: Create Dockerfile (multi-stage)**

`Dockerfile`:
```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o live-sys-stats .

# Runtime stage
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/live-sys-stats .
EXPOSE 8080
ENTRYPOINT ["./live-sys-stats"]
```

**Step 2: Create docker-compose.yml**

`docker-compose.yml`:
```yaml
services:
  live-sys-stats:
    build: .
    ports:
      - "8080:8080"
    network_mode: host          # needed for accurate network stats
    pid: host                   # needed for accurate process/uptime stats
    volumes:
      - /proc:/proc:ro
      - /sys:/sys:ro
    restart: unless-stopped
```

**Step 3: Build and run with Docker**

```bash
docker compose up --build
```
Expected: container starts, logs show `listening on :8080`

**Step 4: Verify the stream works through Docker**

Open `http://localhost:8080/sig.png` — stats should update live.

**Step 5: Stop and commit**

```bash
docker compose down
git add Dockerfile docker-compose.yml
git commit -m "feat: add multi-stage Dockerfile and docker-compose with host mounts"
```

---

## Task 7: Run all tests and final smoke test

**Step 1: Run full test suite**

```bash
go test ./... -v
```
Expected: all tests PASS

**Step 2: Final build verification**

```bash
go build ./...
```
Expected: no errors

**Step 3: Check Docker image size**

```bash
docker compose build
docker images | grep live-sys-stats
```
Expected: image size ~15-25MB

**Step 4: Final commit**

```bash
git add .
git commit -m "chore: verified all tests pass and docker image builds"
```

---

## Quick Reference

| Command | Purpose |
|---|---|
| `go test ./... -v` | Run all tests |
| `go build -o live-sys-stats .` | Build binary |
| `docker compose up --build` | Build + run in Docker |
| `curl -I http://localhost:8080/sig.png` | Check content-type header |

## Forum embed

Once running behind Cloudflare Tunnel:
```
[img]https://<your-tunnel>.trycloudflare.com/sig.png[/img]
```
