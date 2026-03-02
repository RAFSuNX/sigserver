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
