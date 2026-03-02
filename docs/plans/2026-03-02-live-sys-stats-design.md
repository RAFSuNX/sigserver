# Live System Stats Image — Design Doc
**Date:** 2026-03-02

## Overview
A Go service that streams live system stats as a continuously-updating PNG image via `multipart/x-mixed-replace`. Embeddable anywhere an `<img>` tag is supported (forums, GitHub READMEs, etc.) with no JavaScript required.

## Stats Displayed
- **CPU** — usage % + current frequency (GHz)
- **Load Average** — 1m / 5m / 15m
- **RAM** — used / total GB
- **Disk** — used / total GB
- **Network** — live upload MB/s + download MB/s
- **Uptime** — formatted as Xd Xh Xm
- **Hostname** — displayed as header

## Visual Style
- Minimal monospace text layout
- Claude Code theme: dark background (#1a1a1a), off-white text (#e0e0e0), orange accent (#d97706)
- Image size: ~600x160px

## Architecture

### Components
1. **Stats Collector** (`internal/stats/`) — uses `gopsutil` to read all system metrics every 1 second
2. **Image Generator** (`internal/renderer/`) — uses `gg` to draw stats onto a PNG frame
3. **HTTP Server** (`main.go`) — serves `GET /sig.png` as a `multipart/x-mixed-replace` stream

### Data Flow
```
[gopsutil] → StatsCollector (every 1s)
                    ↓
            ImageGenerator → PNG bytes
                    ↓
            HTTP Handler → push frame to all open connections
```

### Streaming Protocol
```
Content-Type: multipart/x-mixed-replace; boundary=frame

--frame
Content-Type: image/png
Content-Length: <n>

<png bytes>
--frame
...
```

## Docker
- Base image: `golang:1.22-alpine` for build, `alpine:3.19` for runtime
- Exposes port `8080`
- Needs host network or volume mounts for accurate disk/network stats:
  - `--net=host` for network stats
  - `-v /proc:/proc:ro` for CPU/uptime
  - `-v /sys:/sys:ro` for CPU frequency

## File Structure
```
.
├── main.go
├── internal/
│   ├── stats/
│   │   └── collector.go
│   └── renderer/
│       └── image.go
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

## Key Dependencies
- `github.com/shirou/gopsutil/v3` — system stats
- `github.com/fogleman/gg` — 2D image drawing
- `golang.org/x/image/font/basicfont` — monospace font
