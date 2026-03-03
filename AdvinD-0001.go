package main

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"math/rand"
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	textv2 "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

const (
	screenW   = 640
	screenH   = 480
	numPoints = 640
	pixelSize = 2

	titleFontSize       = 32
	titleSlideFrames    = 90
	titleShineFrames    = 25
	titleColorFadeFrames = 45
	titleFadeFrames      = 120
	oscilloscopeFlatFrames = 60
	shineBandW           = 70
	shineGradientH   = 48
	shinePeakAlpha    = 0.55
)

var (
	_shineGradient     *ebiten.Image
	_shineGradientOnce sync.Once
	_shineMaskShader   *ebiten.Shader
	_shineShaderOnce   sync.Once
)

func getShineGradientImage() *ebiten.Image {
	_shineGradientOnce.Do(func() {
		img := image.NewRGBA(image.Rect(0, 0, shineBandW, shineGradientH))
		for x := 0; x < shineBandW; x++ {
			t := math.Pi * float64(x) / float64(shineBandW)
			alpha := uint8(255 * shinePeakAlpha * math.Sin(t))
			for y := 0; y < shineGradientH; y++ {
				img.SetRGBA(x, y, color.RGBA{R: 0xff, G: 0xff, B: 0xf8, A: alpha})
			}
		}
		_shineGradient = ebiten.NewImageFromImage(img)
	})
	return _shineGradient
}

const shineMaskShaderSrc = `//kage:unit pixels
package main
func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	grad := imageSrc0At(srcPos)
	title := imageSrc1At(srcPos)
	a := grad.a * title.a
	return vec4(grad.r*a, grad.g*a, grad.b*a, a)
}
`

func getShineMaskShader() *ebiten.Shader {
	_shineShaderOnce.Do(func() {
		var err error
		_shineMaskShader, err = ebiten.NewShader([]byte(shineMaskShaderSrc))
		if err != nil {
			panic(err)
		}
	})
	return _shineMaskShader
}

func titlePhaseTotalFrames() int {
	return titleSlideFrames + titleShineFrames + titleColorFadeFrames + titleFadeFrames
}

const titleText = "Adventures in Demosceneland"

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

var (
	_titleFaceSource *textv2.GoTextFaceSource
	_useV2Text       bool
)

func init() {
	for _, path := range []string{"assets/fonts/Audiowide-Regular.ttf", "assets/fonts/font.ttf", "assets/fonts/Orbitron-Medium.ttf", "font.ttf"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		src, err := textv2.NewGoTextFaceSource(bytes.NewReader(data))
		if err != nil {
			continue
		}
		_titleFaceSource = src
		_useV2Text = true
		break
	}
}

type game struct {
	buf          []float64
	phase        float64
	titleFrame   int
	titleSlideT  float64
	titleShineT  float64
	titleW       float64
	titleH       float64
	titleLayer   *ebiten.Image
	titlePatch   *ebiten.Image
}

func newGame() *game {
	buf := make([]float64, numPoints)
	g := &game{
		buf:        buf,
		titleLayer: ebiten.NewImage(screenW, screenH),
		titlePatch: ebiten.NewImage(shineBandW, shineGradientH),
	}
	if _useV2Text {
		face := &textv2.GoTextFace{Source: _titleFaceSource, Size: titleFontSize}
		g.titleW, g.titleH = textv2.Measure(titleText, face, titleFontSize*1.2)
	} else {
		b := font.MeasureString(basicfont.Face7x13, titleText)
		g.titleW = float64(b.Ceil())
		g.titleH = float64(basicfont.Face7x13.Metrics().Height.Ceil())
	}
	return g
}

func (g *game) reset() {
	g.titleFrame = 0
	g.titleSlideT = 0
	g.titleShineT = 0
	g.phase = 0
	for i := range g.buf {
		g.buf[i] = 0
	}
}

func (g *game) Update() error {
	if ebiten.IsKeyPressed(ebiten.KeyControl) && ebiten.IsKeyPressed(ebiten.KeyShift) && inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.reset()
		return nil
	}
	g.phase += 0.1
	total := titlePhaseTotalFrames()
	if g.titleFrame > total+oscilloscopeFlatFrames {
		g.buf[0] = (rand.Float64() - 0.5) * 0.25
		for i := 1; i < numPoints; i++ {
			delta := (rand.Float64() - 0.5) * 0.08
			g.buf[i] = g.buf[i-1] + delta
		}
	}
	g.titleFrame++
	if g.titleFrame <= titleSlideFrames {
		t := float64(g.titleFrame) / titleSlideFrames
		g.titleSlideT = t * t * (3 - 2*t)
	}
	if g.titleFrame > titleSlideFrames && g.titleFrame <= titleSlideFrames+titleShineFrames {
		g.titleShineT = float64(g.titleFrame-titleSlideFrames) / titleShineFrames
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)
	if g.titleFrame > titlePhaseTotalFrames() {
		g._drawOscilloscope(screen)
	} else {
		g._drawTitle(screen)
	}
}

func (g *game) _drawOscilloscope(screen *ebiten.Image) {
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

func (g *game) _drawTitle(screen *ebiten.Image) {
	titleY := (float64(screenH)-g.titleH)/2
	centerX := float64(screenW) / 2
	startX := -g.titleW - 20
	endX := centerX - g.titleW/2
	titleX := startX + g.titleSlideT*(endX-startX)

	const titleDarkness = 0.12
	colorFadeInStart := titleSlideFrames + titleShineFrames
	fadeToBlackStart := colorFadeInStart + titleColorFadeFrames

	colorBrightness := titleDarkness
	if g.titleFrame > colorFadeInStart && g.titleFrame <= fadeToBlackStart {
		t := float64(g.titleFrame-colorFadeInStart) / float64(titleColorFadeFrames)
		t = t * t * (3 - 2*t)
		colorBrightness = titleDarkness + (1.0-titleDarkness)*t
	} else if g.titleFrame > fadeToBlackStart {
		colorBrightness = 1.0
	}

	fadeT := 0.0
	if g.titleFrame > fadeToBlackStart && g.titleFrame <= fadeToBlackStart+titleFadeFrames {
		fadeT = float64(g.titleFrame-fadeToBlackStart) / float64(titleFadeFrames)
	}
	alphaScale := 1.0 - fadeT

	baseClr := yellowShade(0, g.phase)
	baseClr.R = uint8(float64(baseClr.R) * colorBrightness * (1 - fadeT))
	baseClr.G = uint8(float64(baseClr.G) * colorBrightness * (1 - fadeT))
	baseClr.B = uint8(float64(baseClr.B) * colorBrightness * (1 - fadeT))

	g.titleLayer.Fill(color.RGBA{A: 0})
	if _useV2Text {
		op := &textv2.DrawOptions{}
		op.GeoM.Translate(titleX, titleY)
		op.ColorScale.ScaleWithColor(baseClr)
		face := &textv2.GoTextFace{Source: _titleFaceSource, Size: titleFontSize}
		textv2.Draw(g.titleLayer, titleText, face, op)
	} else {
		textDrawLegacy(g.titleLayer, titleText, int(titleX), int(titleY), baseClr)
	}
	screen.DrawImage(g.titleLayer, nil)

	if g.titleShineT > 0 && g.titleShineT <= 1 {
		shineX := titleX + g.titleShineT*(g.titleW+float64(shineBandW)) - float64(shineBandW)
		shineY := titleY - 4
		g.titlePatch.Fill(color.RGBA{A: 0})
		patchOp := &ebiten.DrawImageOptions{}
		patchOp.GeoM.Translate(-shineX, -shineY)
		g.titlePatch.DrawImage(g.titleLayer, patchOp)
		grad := getShineGradientImage()
		shader := getShineMaskShader()
		op := &ebiten.DrawRectShaderOptions{}
		op.GeoM.Translate(shineX, shineY)
		op.CompositeMode = ebiten.CompositeModeLighter
		op.ColorScale.ScaleAlpha(float32(alphaScale))
		op.Images[0] = grad
		op.Images[1] = g.titlePatch
		screen.DrawRectShader(shineBandW, shineGradientH, shader, op)
	}
}

func textDrawLegacy(screen *ebiten.Image, msg string, x, y int, clr color.Color) {
	text.Draw(screen, msg, basicfont.Face7x13, x, y, clr)
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
