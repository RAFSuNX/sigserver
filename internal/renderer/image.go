package renderer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"live-sys-stats/internal/stats"
)

const (
	imgW = 600
	imgH = 160
)

var (
	bgColor     = color.RGBA{R: 26, G: 26, B: 26, A: 255}   // #1a1a1a
	textColor   = color.RGBA{R: 224, G: 224, B: 224, A: 255} // #e0e0e0
	accentColor = color.RGBA{R: 217, G: 119, B: 6, A: 255}   // #d97706
	dimColor    = color.RGBA{R: 100, G: 100, B: 100, A: 255} // dim separator
)

// Render draws a stats snapshot onto a 600x160 PNG and returns the bytes.
func Render(s stats.Stats) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	draw.Draw(img, img.Bounds(), image.NewUniform(bgColor), image.Point{}, draw.Src)

	// Hostname header in orange
	drawText(img, s.Hostname, 10, 20, accentColor)

	// Separator line
	drawHLine(img, 10, imgW-10, 28, dimColor)

	// Stats rows
	drawText(img, fmt.Sprintf("CPU   %5.1f%%  @  %.2f GHz    load: %.2f  %.2f  %.2f",
		s.CPUPercent, s.CPUFreqGHz, s.LoadAvg[0], s.LoadAvg[1], s.LoadAvg[2]),
		10, 48, textColor)

	drawText(img, fmt.Sprintf("RAM   %.1f / %.1f GB",
		s.RAMUsedGB, s.RAMTotalGB),
		10, 68, textColor)

	drawText(img, fmt.Sprintf("DISK  %.1f / %.1f GB",
		s.DiskUsedGB, s.DiskTotalGB),
		10, 88, textColor)

	drawText(img, fmt.Sprintf("NET   up %.2f MB/s    down %.2f MB/s",
		s.NetUpMBps, s.NetDownMBps),
		10, 108, textColor)

	drawText(img, fmt.Sprintf("UP    %s", s.UptimeStr),
		10, 128, textColor)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

func drawHLine(img *image.RGBA, x0, x1, y int, clr color.Color) {
	for x := x0; x <= x1; x++ {
		img.Set(x, y, clr)
	}
}
