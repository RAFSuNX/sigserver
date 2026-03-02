package stats_test

import (
	"testing"
	"time"
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
	if _, err := c.Collect(); err != nil {
		t.Fatalf("prime call failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	s, err := c.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.NetUpMBps < 0 {
		t.Errorf("net up rate should be >= 0, got %f", s.NetUpMBps)
	}
	if s.NetDownMBps < 0 {
		t.Errorf("net down rate should be >= 0, got %f", s.NetDownMBps)
	}
	if s.NetUpMBps > 10000 {
		t.Errorf("net up rate suspiciously high (counter underflow?): %f", s.NetUpMBps)
	}
	if s.NetDownMBps > 10000 {
		t.Errorf("net down rate suspiciously high (counter underflow?): %f", s.NetDownMBps)
	}
}
