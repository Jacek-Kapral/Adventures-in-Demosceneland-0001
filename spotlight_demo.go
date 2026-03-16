// spotlight_demo.go — 7 snopów ze środka ekranu, losowe błyski, różne kolory. go run spotlight_demo.go
package main

import (
	"image/color"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	screenW        = 640
	screenH        = 480
	numBeams       = 7
	beamLen        = 400
	beamHalfAngle  = 12.0 * math.Pi / 180
	softAngleOut   = 2.2
	softLenOut     = 1.25
	fadePerFrame   = 0.965
	spreadPerFrame = 0.28
)

var spotlightColors = []struct{ R, G, B uint8 }{
	{255, 255, 255},
	{255, 100, 80},
	{80, 80, 255},
	{80, 255, 120},
	{255, 200, 60},
	{255, 80, 200},
	{80, 255, 255},
}

type beam struct {
	angle       float64
	colorIndex  int
	intensity   float64
	spread      float64
	nextFlashAt float64
}

type spotlightGame struct {
	time      float64
	beams     [numBeams]beam
	buf       []byte
	offscreen *ebiten.Image
}

func smoothstep(edge0, edge1, x float64) float64 {
	if x <= edge0 {
		return 0
	}
	if x >= edge1 {
		return 1
	}
	t := (x - edge0) / (edge1 - edge0)
	return t * t * (3 - 2*t)
}

func (g *spotlightGame) Update() error {
	g.time += 1.0 / 60.0
	for i := range g.beams {
		b := &g.beams[i]
		if g.time >= b.nextFlashAt {
			b.intensity = 1.0
			b.spread = 0
			b.colorIndex = rand.Intn(len(spotlightColors))
			b.nextFlashAt = g.time + 1.2 + rand.Float64()*2.8
		}
		if b.spread < 1 {
			b.spread += spreadPerFrame
			if b.spread > 1 {
				b.spread = 1
			}
		}
		b.intensity *= fadePerFrame
		if b.intensity < 0.002 {
			b.intensity = 0
		}
	}
	return nil
}

func (g *spotlightGame) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 8, G: 6, B: 12, A: 255})
	cx := float64(screenW) / 2
	cy := float64(screenH) / 2
	const bgR, bgG, bgB = 8, 6, 12
	const brightnessBoost = 1.4
	for y := 0; y < screenH; y++ {
		for x := 0; x < screenW; x++ {
			idx := (y*screenW + x) * 4
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Sqrt(dx*dx + dy*dy)
			angle := math.Atan2(dy, dx)
			var bestR, bestG, bestB float64 = float64(bgR), float64(bgG), float64(bgB)
			bestFactor := 0.0
			for i := range g.beams {
				b := &g.beams[i]
				relAngle := angle - b.angle
				for relAngle > math.Pi {
					relAngle -= 2 * math.Pi
				}
				for relAngle < -math.Pi {
					relAngle += 2 * math.Pi
				}
				distNorm := dist / beamLen
				radialFalloff := 1.0 - smoothstep(b.spread*0.9, b.spread*1.05, distNorm)
				angleFalloff := 1.0 - smoothstep(beamHalfAngle*0.3, beamHalfAngle*softAngleOut, math.Abs(relAngle))
				lenFalloff := 1.0 - distNorm*0.4
				softLenFalloff := 1.0 - smoothstep(beamLen*0.6, beamLen*softLenOut, dist)
				factor := b.intensity * radialFalloff * angleFalloff * lenFalloff * softLenFalloff
				if factor <= bestFactor {
					continue
				}
				bestFactor = factor
				if factor > 1 {
					factor = 1
				}
				c := &spotlightColors[b.colorIndex]
				bestR = float64(bgR) + (float64(c.R)-float64(bgR))*factor*brightnessBoost
				bestG = float64(bgG) + (float64(c.G)-float64(bgG))*factor*brightnessBoost
				bestB = float64(bgB) + (float64(c.B)-float64(bgB))*factor*brightnessBoost
			}
			g.buf[idx] = uint8(math.Min(255, math.Max(0, bestR)))
			g.buf[idx+1] = uint8(math.Min(255, math.Max(0, bestG)))
			g.buf[idx+2] = uint8(math.Min(255, math.Max(0, bestB)))
			g.buf[idx+3] = 255
		}
	}
	g.offscreen.ReplacePixels(g.buf)
	screen.DrawImage(g.offscreen, nil)
}

func (g *spotlightGame) Layout(outW, outH int) (int, int) {
	return screenW, screenH
}

func main() {
	g := &spotlightGame{
		buf:       make([]byte, screenW*screenH*4),
		offscreen: ebiten.NewImage(screenW, screenH),
	}
	for i := range g.beams {
		g.beams[i].angle = 2 * math.Pi * float64(i) / numBeams
		g.beams[i].colorIndex = i % len(spotlightColors)
		g.beams[i].nextFlashAt = g.beams[i].angle*0.3 + rand.Float64()*1.5
	}
	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("Spotlight")
	if err := ebiten.RunGame(g); err != nil {
		panic(err)
	}
}
