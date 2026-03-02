package renderer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"time"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"live-sys-stats/internal/stats"
)

const (
	imgW = 600
	imgH = 300
	lx   = 0   // left margin
	barX = 72  // bar left edge ("CPU" ~3 chars at ~24px each)
	barW = 300 // bar width
	barH = 14  // bar height
	vx   = 378 // value text x (barX + barW + 6)
)

var (
	textColor  = color.RGBA{R: 224, G: 224, B: 224, A: 255}
	accent     = color.RGBA{R: 217, G: 119, B: 6, A: 255}
	dimColor   = color.RGBA{R: 100, G: 100, B: 100, A: 255}
	subColor   = color.RGBA{R: 156, G: 163, B: 175, A: 255}
	barBgColor = color.RGBA{R: 45, G: 45, B: 45, A: 210}
	barFill    = color.RGBA{R: 217, G: 119, B: 6, A: 255}
	barHot     = color.RGBA{R: 239, G: 68, B: 68, A: 255}
	green      = color.RGBA{R: 34, G: 197, B: 94, A: 255}
	blue       = color.RGBA{R: 96, G: 165, B: 250, A: 255}
)

// face is the parsed Go Mono font face at 13pt, initialised once.
var face xfont.Face

func init() {
	tt, err := opentype.Parse(gomono.TTF)
	if err != nil {
		log.Fatalf("parse font: %v", err)
	}
	face, err = opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    13,
		DPI:     96,
		Hinting: xfont.HintingFull,
	})
	if err != nil {
		log.Fatalf("new face: %v", err)
	}
}

// charW returns approximate advance width for n chars (Go Mono is monospaced).
func charW(n int) int {
	adv, _ := face.GlyphAdvance('M')
	return (adv.Round() * n)
}

// Render draws a stats snapshot onto a 600x300 transparent PNG.
func Render(s stats.Stats) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))

	// ── Header ────────────────────────────────────────────────────────
	dotColor := accent
	if time.Now().Second()%2 != 0 {
		dotColor = color.RGBA{}
	}
	drawText(img, s.Hostname, lx, 17, accent)
	hw := charW(len(s.Hostname))
	fillRect(img, lx+hw+4, 5, 8, 8, dotColor)

	cpuInfo := s.CPUModel
	if s.CPUFreqGHz > 0 {
		cpuInfo = fmt.Sprintf("%.2f GHz", s.CPUFreqGHz)
	}
	if cpuInfo != "" {
		drawTextRight(img, cpuInfo, imgW, 17, subColor)
	}

	// 2px orange separator
	fillRect(img, lx, 24, imgW, 2, accent)

	// ── CPU (y=28) ────────────────────────────────────────────────────
	drawText(img, "CPU", lx, 42, accent)
	drawBar(img, barX, 29, barW, barH, s.CPUPercent/100.0)
	drawText(img, fmt.Sprintf("%.1f%%", s.CPUPercent), vx, 42, textColor)
	loadStr := fmt.Sprintf("%.2f %.2f %.2f", s.LoadAvg[0], s.LoadAvg[1], s.LoadAvg[2])
	drawTextRight(img, loadStr, imgW, 42, subColor)

	// ── RAM (y=72) ────────────────────────────────────────────────────
	drawText(img, "RAM", lx, 87, accent)
	ramPct := 0.0
	if s.RAMTotalGB > 0 {
		ramPct = s.RAMUsedGB / s.RAMTotalGB
	}
	drawBar(img, barX, 74, barW, barH, ramPct)
	drawText(img, fmt.Sprintf("%.1f/%.1fGB", s.RAMUsedGB, s.RAMTotalGB), vx, 87, textColor)

	// ── DISK (y=116) ──────────────────────────────────────────────────
	drawText(img, "DSK", lx, 131, accent)
	diskPct := 0.0
	if s.DiskTotalGB > 0 {
		diskPct = s.DiskUsedGB / s.DiskTotalGB
	}
	drawBar(img, barX, 118, barW, barH, diskPct)
	drawText(img, fmt.Sprintf("%.0f/%.0fGB", s.DiskUsedGB, s.DiskTotalGB), vx, 131, textColor)

	// ── Dim separator ─────────────────────────────────────────────────
	fillRect(img, lx, 140, imgW, 1, dimColor)

	// ── NET (y=142) ───────────────────────────────────────────────────
	drawText(img, "NET", lx, 162, accent)
	drawTriangleUp(img, barX, 150, green)
	drawText(img, fmt.Sprintf("%.3f MB/s", s.NetUpMBps), barX+18, 162, textColor)
	drawTriangleDown(img, barX+195, 150, blue)
	drawText(img, fmt.Sprintf("%.3f MB/s", s.NetDownMBps), barX+213, 162, textColor)

	// ── UPTIME (y=188) ────────────────────────────────────────────────
	drawText(img, "UP", lx, 194, accent)
	drawText(img, s.UptimeStr, barX, 194, textColor)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawBar(img *image.RGBA, x, y, w, h int, pct float64) {
	fillRect(img, x, y, w, h, barBgColor)
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	if fill := int(float64(w) * pct); fill > 0 {
		fc := barFill
		if pct >= 0.9 {
			fc = barHot
		}
		fillRect(img, x, y, fill, h, fc)
	}
}

func fillRect(img *image.RGBA, x, y, w, h int, clr color.Color) {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			img.Set(x+dx, y+dy, clr)
		}
	}
}

func drawText(img *image.RGBA, text string, x, y int, clr color.Color) {
	d := &xfont.Drawer{
		Dst:  img,
		Src:  image.NewUniform(clr),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(text)
}

func drawTextRight(img *image.RGBA, text string, rightX, y int, clr color.Color) {
	drawText(img, text, rightX-charW(len(text)), y, clr)
}

func drawTriangleUp(img *image.RGBA, x, y int, clr color.Color) {
	for row := 0; row < 7; row++ {
		cx := x + 6
		for i := -row; i <= row; i++ {
			img.Set(cx+i, y+(6-row), clr)
		}
	}
}

func drawTriangleDown(img *image.RGBA, x, y int, clr color.Color) {
	for row := 0; row < 7; row++ {
		cx := x + 6
		half := 6 - row
		for i := -half; i <= half; i++ {
			img.Set(cx+i, y+row, clr)
		}
	}
}
