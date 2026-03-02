package stats

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	CPUModel    string
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

	// CPU frequency + model name
	freqs, _ := cpu.Info()
	cpuFreqGHz := 0.0
	cpuModel := ""
	if len(freqs) > 0 {
		cpuFreqGHz = freqs[0].Mhz / 1000.0
		cpuModel = freqs[0].ModelName
	}
	if cpuModel == "" {
		cpuModel = cpuModelFromProcInfo()
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

	// Disk — sum every real partition visible (host mount namespace via /proc/1)
	virtualFS := map[string]bool{
		"tmpfs": true, "devtmpfs": true, "sysfs": true, "proc": true,
		"devpts": true, "cgroup": true, "cgroup2": true, "pstore": true,
		"mqueue": true, "hugetlbfs": true, "debugfs": true, "tracefs": true,
		"securityfs": true, "configfs": true, "fusectl": true, "bpf": true,
		"efivarfs": true, "squashfs": true, "overlay": true, "ramfs": true,
		"autofs": true, "nsfs": true,
	}
	var diskUsedBytes, diskTotalBytes uint64
	parts, err := disk.Partitions(true)
	if err != nil {
		return nil, fmt.Errorf("disk partitions: %w", err)
	}
	seenDev := map[string]bool{}
	for _, p := range parts {
		if virtualFS[p.Fstype] {
			continue
		}
		if seenDev[p.Device] {
			continue
		}
		seenDev[p.Device] = true
		u, derr := disk.Usage(p.Mountpoint)
		if derr != nil {
			continue
		}
		diskUsedBytes += u.Used
		diskTotalBytes += u.Total
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
		CPUModel:    cpuModel,
		LoadAvg:     [3]float64{avg.Load1, avg.Load5, avg.Load15},
		RAMUsedGB:   float64(vmem.Used) / 1024 / 1024 / 1024,
		RAMTotalGB:  float64(vmem.Total) / 1024 / 1024 / 1024,
		DiskUsedGB:  float64(diskUsedBytes) / 1024 / 1024 / 1024,
		DiskTotalGB: diskTotalGB(diskTotalBytes),
		NetUpMBps:   netUpMBps,
		NetDownMBps: netDownMBps,
		UptimeStr:   uptimeStr,
	}, nil
}

// cpuModelFromProcInfo decodes ARM CPU implementer+part from /proc/cpuinfo.
func cpuModelFromProcInfo() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	var implementer, part string
	for _, line := range strings.Split(string(data), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok {
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			switch k {
			case "CPU implementer":
				implementer = v
			case "CPU part":
				part = v
			}
		}
		if implementer != "" && part != "" {
			break
		}
	}
	// ARM Ltd (0x41) part lookup
	if implementer == "0x41" {
		armParts := map[string]string{
			"0xd03": "Cortex-A53",
			"0xd04": "Cortex-A35",
			"0xd05": "Cortex-A55",
			"0xd07": "Cortex-A57",
			"0xd08": "Cortex-A72",
			"0xd09": "Cortex-A73",
			"0xd0a": "Cortex-A75",
			"0xd0b": "Cortex-A76",
			"0xd0c": "Neoverse N1",
			"0xd0d": "Cortex-A77",
			"0xd40": "Neoverse V1",
			"0xd41": "Cortex-A78",
			"0xd49": "Neoverse N2",
			"0xd4a": "Neoverse E1",
		}
		if name, ok := armParts[part]; ok {
			return name
		}
	}
	if implementer != "" && part != "" {
		return fmt.Sprintf("ARM %s/%s", implementer, part)
	}
	return ""
}

func diskTotalGB(detectedBytes uint64) float64 {
	if v := os.Getenv("DISK_TOTAL_GB"); v != "" {
		if gb, err := strconv.ParseFloat(v, 64); err == nil && gb > 0 {
			return gb
		}
	}
	return float64(detectedBytes) / 1024 / 1024 / 1024
}

// FormatUptime formats total seconds as d:hh:mm:ss.
func FormatUptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return fmt.Sprintf("%d:%02d:%02d:%02d", days, hours, minutes, secs)
}
