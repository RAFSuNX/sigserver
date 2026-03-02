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
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("hostname: %w", err)
	}

	// CPU percent (100ms interval for accuracy)
	percents, err := cpu.Percent(100*time.Millisecond, false)
	if err != nil {
		return nil, fmt.Errorf("cpu percent: %w", err)
	}
	cpuPct := 0.0
	if len(percents) > 0 {
		cpuPct = percents[0]
	}

	// CPU frequency (nominal base clock from /proc/cpuinfo)
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
	if err != nil || len(netCounters) == 0 {
		c.prevNetTime = time.Time{} // force re-prime on next success
	} else {
		sent := netCounters[0].BytesSent
		recv := netCounters[0].BytesRecv
		if !c.prevNetTime.IsZero() {
			elapsed := now.Sub(c.prevNetTime).Seconds()
			if elapsed > 0 {
				// guard against counter reset/wraparound
				if sent >= c.prevNetBytes[0] && recv >= c.prevNetBytes[1] {
					netUpMBps = float64(sent-c.prevNetBytes[0]) / elapsed / 1024 / 1024
					netDownMBps = float64(recv-c.prevNetBytes[1]) / elapsed / 1024 / 1024
				}
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
	uptimeStr := FormatUptime(uptime)

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

// FormatUptime formats total seconds as d:hh:mm:ss.
func FormatUptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return fmt.Sprintf("%d:%02d:%02d:%02d", days, hours, minutes, secs)
}
