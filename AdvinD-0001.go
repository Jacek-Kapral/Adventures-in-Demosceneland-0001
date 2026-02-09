package main

import (
	"image/color"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	screenW   = 640
	screenH   = 480
	numPoints = 320
)

var (
	yellow = color.RGBA{R: 0xff, G: 0xff, B: 0x00, A: 0xff}
)

type game struct {
	buf   []float64
	phase float64
}

func newGame() *game {
	buf := make([]float64, numPoints)
	for i := range buf {
		buf[i] = 0
	}
	return &game{buf: buf}
}

func (g *game) Update() error {
	g.phase += 0.1
	for i := range g.buf {
		g.buf[i] = (rand.Float64()-0.5)*0.8 + math.Sin(g.phase+float64(i)*0.05)*0.2
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)
	midY := float64(screenH) / 2
	scaleY := float64(screenH) * 0.35
	stepX := float64(screenW) / float64(numPoints-1)
	for i := 0; i < numPoints-1; i++ {
		x0 := float64(i) * stepX
		y0 := midY - g.buf[i]*scaleY
		x1 := float64(i+1) * stepX
		y1 := midY - g.buf[i+1]*scaleY
		ebitenutil.DrawLine(screen, x0, y0, x1, y1, yellow)
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
