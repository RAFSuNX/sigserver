# Cloudflare Tunnel + Uptime Format — Design Doc
**Date:** 2026-03-02

## Changes

### 1. Uptime Format
Change `formatUptime()` in `internal/stats/collector.go` from `3d 14h 22m` to `3:14:22:45` (d:h:m:s).

### 2. Cloudflare Temporary Tunnel (Option A — Go subprocess)

**Behaviour:**
- On startup, `main.go` checks if `cloudflared` is in PATH
- If found: spawns `cloudflared tunnel --url http://localhost:8080` as a child process
- Scans cloudflared's stderr for a line containing `trycloudflare.com`
- Once URL is detected, prints:
  ```
  ════════════════════════════════════════
  SYSTEM ONLINE! PASTE THIS INTO YOUR FORUM:
  [img]https://xxxx.trycloudflare.com/sig.png[/img]
  ════════════════════════════════════════
  ```
- If `cloudflared` not found: logs a warning, continues running without tunnel
- On SIGTERM: child process is killed before main process exits

**Files changed:**
- `internal/stats/collector.go` — uptime format (d:h:m:s with colons)
- `internal/stats/collector_test.go` — update uptime format assertion
- `main.go` — optional cloudflared subprocess + URL detection
- `Dockerfile` — install cloudflared binary in runtime image

**Tunnel URL detection:**
Cloudflared prints a line like:
```
Your quick Tunnel has been created! Visit it at (it may take some time to be reachable):
https://xxxx.trycloudflare.com
```
Scan stderr lines for `trycloudflare.com`, extract the URL with a simple string search.
