package main

import (
	"image/color"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	screenW       = 640
	screenH       = 480
	numPoints     = 320
	pixelSize     = 2
	numPolyGroups = 5
	polyStrokeW   = 1.5
	polyMinSides  = 5
	polyMaxSides  = 10
)

var polyGrey = color.RGBA{R: 0xb0, G: 0xb0, B: 0xb0, A: 0x55}

func quantize(v float64, grid int) float64 {
	return float64(int(v/float64(grid)) * grid)
}

func yellowShade(segmentIndex int, phase float64) color.RGBA {
	t := float64(segmentIndex)*0.02 + phase*0.5
	bright := 0.6 + 0.4*math.Sin(t)
	r := uint8(255 * bright)
	g := uint8((200 + 55*math.Cos(t*0.7)) * bright)
	return color.RGBA{R: r, G: g, B: 0x00, A: 0xff}
}

type polyGroup struct {
	verts     [][2]float64
	x, y      float64
	sx, sy    float64
	vx, vy    float64
}

type game struct {
	buf        []float64
	phase      float64
	polyGroups []polyGroup
}

func newPolyGroup(seed int64) polyGroup {
	rng := rand.New(rand.NewSource(seed))
	x := rng.Float64() * screenW
	y := rng.Float64() * screenH
	sx := 30 + rng.Float64()*50
	sy := 30 + rng.Float64()*50
	n := polyMinSides + rng.Intn(polyMaxSides-polyMinSides+1)
	verts := make([][2]float64, n)
	for i := 0; i < n; i++ {
		angle := 2*math.Pi*float64(i)/float64(n) + (rng.Float64()-0.5)*0.8
		r := 0.4 + rng.Float64()*0.4
		verts[i] = [2]float64{0.5 + r*math.Cos(angle), 0.5 + r*math.Sin(angle)}
	}
	return polyGroup{
		verts: verts,
		x: x, y: y, sx: sx, sy: sy,
		vx: (rng.Float64() - 0.5) * 0.8,
		vy: (rng.Float64() - 0.5) * 0.8,
	}
}

func newGame() *game {
	buf := make([]float64, numPoints)
	for i := range buf {
		buf[i] = 0
	}
	groups := make([]polyGroup, numPolyGroups)
	for i := range groups {
		groups[i] = newPolyGroup(int64(i * 12345))
	}
	return &game{buf: buf, polyGroups: groups}
}

func (g *game) Update() error {
	g.phase += 0.1
	g.buf[0] = (rand.Float64() - 0.5) * 0.25
	for i := 1; i < len(g.buf); i++ {
		delta := (rand.Float64() - 0.5) * 0.08
		g.buf[i] = g.buf[i-1] + delta
	}
	for i := range g.polyGroups {
		p := &g.polyGroups[i]
		p.x += p.vx
		p.y += p.vy
		if p.x < -100 || p.x > screenW+100 {
			p.vx = -p.vx
		}
		if p.y < -100 || p.y > screenH+100 {
			p.vy = -p.vy
		}
		p.sx *= 1 + (rand.Float64()-0.5)*0.04
		p.sy *= 1 + (rand.Float64()-0.5)*0.04
		if p.sx < 20 {
			p.sx = 20
		}
		if p.sx > 100 {
			p.sx = 100
		}
		if p.sy < 20 {
			p.sy = 20
		}
		if p.sy > 100 {
			p.sy = 100
		}
	}
	return nil
}

func (g *game) drawPolyOutline(screen *ebiten.Image, p *polyGroup) {
	n := len(p.verts)
	if n < 2 {
		return
	}
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		a, b := p.verts[i], p.verts[j]
		ax := float32(p.x + a[0]*p.sx)
		ay := float32(p.y + a[1]*p.sy)
		bx := float32(p.x + b[0]*p.sx)
		by := float32(p.y + b[1]*p.sy)
		vector.StrokeLine(screen, ax, ay, bx, by, float32(polyStrokeW), polyGrey, false)
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)
	for i := range g.polyGroups {
		g.drawPolyOutline(screen, &g.polyGroups[i])
	}
	midY := float64(screenH) / 2
	scaleY := float64(screenH) * 0.35
	stepX := float64(screenW) / float64(numPoints-1)
	for i := 0; i < numPoints; i++ {
		x := quantize(float64(i)*stepX, pixelSize)
		y := quantize(midY-g.buf[i]*scaleY, pixelSize)
		clr := yellowShade(i, g.phase)
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(pixelSize), float32(pixelSize), clr, false)
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
