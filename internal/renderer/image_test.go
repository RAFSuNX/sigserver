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
	s := stats.Stats{
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
	if bounds.Dx() != 700 || bounds.Dy() != 300 {
		t.Errorf("expected 700x300, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Verify background is transparent at top-left corner
	_, _, _, a := img.At(0, 0).RGBA()
	if a != 0 {
		t.Errorf("expected transparent background at (0,0), got alpha %d", a>>8)
	}

	// Verify at least one non-transparent pixel exists (text was rendered)
	anyRendered := false
	imgBounds := img.Bounds()
outer:
	for py := imgBounds.Min.Y; py < imgBounds.Max.Y; py++ {
		for px := imgBounds.Min.X; px < imgBounds.Max.X; px++ {
			_, _, _, pa := img.At(px, py).RGBA()
			if pa != 0 {
				anyRendered = true
				break outer
			}
		}
	}
	if !anyRendered {
		t.Error("all pixels are transparent — no text was rendered")
	}
}
