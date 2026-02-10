package main

import (
	"image/color"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	screenW    = 640
	screenH    = 480
	numPoints  = 320
	pixelSize  = 8
	lineStrokeW = 1.5
)

func quantize(v float64, grid int) float64 {
	return float64(int(v/float64(grid)) * grid)
}

func yellowShade(segmentIndex int, phase float64) color.RGBA {
	t := float64(segmentIndex)*0.02 + phase*0.5
	bright := 0.5 + 0.55*math.Sin(t)
	if bright < 0.5 {
		bright = 0.5
	}
	r := uint8(math.Min(255, 255*bright))
	g := uint8(math.Min(255, (200+55*math.Cos(t*0.7))*bright))
	return color.RGBA{R: r, G: g, B: 0x00, A: 0xff}
}

type game struct {
	buf   []float64
	phase float64
}

func newGame() *game {
	buf := make([]float64, numPoints)
	return &game{buf: buf}
}

func (g *game) Update() error {
	g.phase += 0.1
	g.buf[0] = (rand.Float64() - 0.5) * 0.25
	for i := 1; i < numPoints; i++ {
		delta := (rand.Float64() - 0.5) * 0.08
		g.buf[i] = g.buf[i-1] + delta
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)
	midY := float64(screenH) / 2
	scaleY := float64(screenH) * 0.35
	stepX := float64(screenW) / float64(numPoints-1)
	for i := 0; i < numPoints-1; i++ {
		x0 := quantize(float64(i)*stepX, pixelSize)
		y0 := quantize(midY-g.buf[i]*scaleY, pixelSize)
		x1 := quantize(float64(i+1)*stepX, pixelSize)
		y1 := quantize(midY-g.buf[i+1]*scaleY, pixelSize)
		clr := yellowShade(i, g.phase)
		vector.StrokeLine(screen, float32(x0), float32(y0), float32(x1), float32(y1), float32(lineStrokeW), clr, false)
	}
}

func (g *game) Layout(outsideW, outsideH int) (int, int) {
	return screenW, screenH
}

func main() {
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("AdvinD-0001 — Oscilloscope")
	if err := ebiten.RunGame(newGame()); err != nil {
		panic(err)
	}
}
