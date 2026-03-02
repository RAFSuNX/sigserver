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
	if bounds.Dx() != 600 || bounds.Dy() != 160 {
		t.Errorf("expected 600x160, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Verify background color at top-left corner (should be #1a1a1a)
	r, g, b, a := img.At(0, 0).RGBA()
	// RGBA() returns 16-bit premultiplied. Shift right 8 to get 8-bit values.
	if r>>8 != 26 || g>>8 != 26 || b>>8 != 26 || a>>8 != 255 {
		t.Errorf("expected background #1a1a1a at (0,0), got R:%d G:%d B:%d A:%d", r>>8, g>>8, b>>8, a>>8)
	}

	// Verify at least one non-background pixel exists (text was rendered)
	anyRendered := false
	imgBounds := img.Bounds()
outer:
	for py := imgBounds.Min.Y; py < imgBounds.Max.Y; py++ {
		for px := imgBounds.Min.X; px < imgBounds.Max.X; px++ {
			pr, pg, pb, _ := img.At(px, py).RGBA()
			if pr>>8 != 26 || pg>>8 != 26 || pb>>8 != 26 {
				anyRendered = true
				break outer
			}
		}
	}
	if !anyRendered {
		t.Error("all pixels are background color — no text was rendered")
	}
}
