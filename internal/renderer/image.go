package renderer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"live-sys-stats/internal/stats"
)

const (
	imgW = 600
	imgH = 300

	lx   = 0   // left margin
	barX = 71  // bar left edge
	barW = 284 // bar width
	barH = 12  // bar height
	vx   = 358 // value text x (barX + barW + 3)
)

var (
	textColor  = color.RGBA{R: 224, G: 224, B: 224, A: 255} // #e0e0e0
	accent     = color.RGBA{R: 217, G: 119, B: 6, A: 255}   // #d97706 orange
	dimColor   = color.RGBA{R: 100, G: 100, B: 100, A: 255} // #646464
	subColor   = color.RGBA{R: 156, G: 163, B: 175, A: 255} // #9ca3af
	barBgColor = color.RGBA{R: 45, G: 45, B: 45, A: 210}    // bar track
	barFill    = color.RGBA{R: 217, G: 119, B: 6, A: 255}   // orange fill
	barHot     = color.RGBA{R: 239, G: 68, B: 68, A: 255}   // red >90%
	green      = color.RGBA{R: 34, G: 197, B: 94, A: 255}   // #22c55e
	blue       = color.RGBA{R: 96, G: 165, B: 250, A: 255}  // #60a5fa
)

// Render draws a stats snapshot onto a 700x300 transparent PNG.
func Render(s stats.Stats) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	// image.NewRGBA zero-initialises to (0,0,0,0) — transparent

	// ── Header ────────────────────────────────────────────────────────
	fillRect(img, lx, 21, 7, 7, accent) // accent square
	drawText(img, " "+s.Hostname, lx+7, 30, accent)

	cpuInfo := s.CPUModel
	if s.CPUFreqGHz > 0 {
		cpuInfo = fmt.Sprintf("%.2f GHz", s.CPUFreqGHz)
	}
	if cpuInfo != "" {
		drawTextRight(img, cpuInfo, imgW-lx, 30, subColor)
	}

	// 2px orange separator
	fillRect(img, lx, 40, imgW-lx*2, 2, accent)

	// ── CPU (bar centre y=67) ──────────────────────────────────────────
	drawText(img, "CPU", lx, 72, accent)
	drawBar(img, barX, 61, barW, barH, s.CPUPercent/100.0)
	drawText(img, fmt.Sprintf("%.1f%%", s.CPUPercent), vx, 72, textColor)
	loadStr := fmt.Sprintf("load  %.2f  %.2f  %.2f", s.LoadAvg[0], s.LoadAvg[1], s.LoadAvg[2])
	drawTextRight(img, loadStr, imgW-lx, 72, subColor)

	// ── RAM (bar centre y=104) ─────────────────────────────────────────
	drawText(img, "RAM", lx, 109, accent)
	ramPct := 0.0
	if s.RAMTotalGB > 0 {
		ramPct = s.RAMUsedGB / s.RAMTotalGB
	}
	drawBar(img, barX, 98, barW, barH, ramPct)
	drawText(img, fmt.Sprintf("%.1f / %.1f GB", s.RAMUsedGB, s.RAMTotalGB), vx, 109, textColor)

	// ── DISK (bar centre y=141) ────────────────────────────────────────
	drawText(img, "DSK", lx, 146, accent)
	diskPct := 0.0
	if s.DiskTotalGB > 0 {
		diskPct = s.DiskUsedGB / s.DiskTotalGB
	}
	drawBar(img, barX, 135, barW, barH, diskPct)
	drawText(img, fmt.Sprintf("%.0f / %.0f GB", s.DiskUsedGB, s.DiskTotalGB), vx, 146, textColor)

	// ── Dim separator ─────────────────────────────────────────────────
	fillRect(img, lx, 165, imgW-lx*2, 1, dimColor)

	// ── NET ───────────────────────────────────────────────────────────
	drawText(img, "NET", lx, 193, accent)
	drawTriangleUp(img, barX, 186, green)
	drawText(img, fmt.Sprintf(" %.2f MB/s", s.NetUpMBps), barX+11, 193, textColor)
	drawTriangleDown(img, barX+170, 186, blue)
	drawText(img, fmt.Sprintf(" %.2f MB/s", s.NetDownMBps), barX+181, 193, textColor)

	// ── UPTIME ────────────────────────────────────────────────────────
	drawText(img, "UP", lx, 228, accent)
	drawText(img, s.UptimeStr, barX, 228, textColor)

	// ── Dim separator ─────────────────────────────────────────────────
	fillRect(img, lx, 248, imgW-lx*2, 1, dimColor)

	// ── Footer ────────────────────────────────────────────────────────
	fillRect(img, lx, 262, 7, 7, green) // green live indicator
	drawText(img, " LIVE", lx+7, 271, green)

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
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(text)
}

// drawTextRight right-aligns text so its right edge is at rightX.
func drawTextRight(img *image.RGBA, text string, rightX, y int, clr color.Color) {
	drawText(img, text, rightX-len(text)*7, y, clr)
}

// drawTriangleUp draws a 9×5 upward-pointing triangle with tip at top.
func drawTriangleUp(img *image.RGBA, x, y int, clr color.Color) {
	for row := 0; row < 5; row++ {
		cx := x + 4
		for i := -row; i <= row; i++ {
			img.Set(cx+i, y+(4-row), clr)
		}
	}
}

// drawTriangleDown draws a 9×5 downward-pointing triangle with tip at bottom.
func drawTriangleDown(img *image.RGBA, x, y int, clr color.Color) {
	for row := 0; row < 5; row++ {
		cx := x + 4
		half := 4 - row
		for i := -half; i <= half; i++ {
			img.Set(cx+i, y+row, clr)
		}
	}
}
