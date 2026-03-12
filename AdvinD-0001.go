package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"math"
	"math/rand"
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
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

	oscilloscopeShorterBy   = 226
	titleFontSize          = 32
	titleSlideFrames       = 90
	titleShineFrames       = 25
	titleColorFadeFrames   = 45
	titleFadeFrames        = 120
	columnsPhaseFrames    = 70
	oscilloscopeFlatFrames = 120
	shineBandW             = 70
	shineGradientH         = 48
	shinePeakAlpha         = 0.55

	audioSampleRate = 48000
	musicPath       = "assets/music/JungleMirage.mp3"

	columnsAlpha = 0.4

	pulseScale = 0.12
)

func oscilloscopeLineWidth() int { return screenW - oscilloscopeShorterBy }
func oscilloscopeMarginX() int   { return oscilloscopeShorterBy / 2 }

var (
	_shineGradient     *ebiten.Image
	_shineGradientOnce sync.Once
	_shineMaskShader   *ebiten.Shader
	_shineShaderOnce   sync.Once

	_columnLeft  *ebiten.Image
	_columnRight *ebiten.Image

	_konsoleFrames []*ebiten.Image
	_advieFrames   []*ebiten.Image
)

func loadColumnImages() {
	fl, err := os.Open("assets/img/column_left_154px.png")
	if err != nil {
		return
	}
	defer fl.Close()
	imgL, err := png.Decode(fl)
	if err != nil {
		return
	}
	fr, err := os.Open("assets/img/column_right_154px.png")
	if err != nil {
		return
	}
	defer fr.Close()
	imgR, err := png.Decode(fr)
	if err != nil {
		return
	}
	_columnLeft = ebiten.NewImageFromImage(imgL)
	_columnRight = ebiten.NewImageFromImage(imgR)
}

func loadGIFFrames(path string) []*ebiten.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		return nil
	}
	frames := make([]*ebiten.Image, 0, len(g.Image))
	for _, src := range g.Image {
		frames = append(frames, ebiten.NewImageFromImage(src))
	}
	return frames
}

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

func oscilloscopeStartFrame() int {
	return titlePhaseTotalFrames() + columnsPhaseFrames
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
	for _, path := range []string{"assets/fonts/RubikGlitch-Regular.ttf", "assets/fonts/font.ttf", "assets/fonts/Orbitron-Medium.ttf", "font.ttf"} {
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
	loadColumnImages()
	_konsoleFrames = loadGIFFrames("assets/img/konsole.gif")
	_advieFrames = loadGIFFrames("assets/img/advie.gif")
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

	audioContext *audio.Context
	musicPlayer  *audio.Player
	musicStarted bool
	pcmBuffer    []byte

	konsoleFrame int
	advieFrame   int
}

func newGame() *game {
	buf := make([]float64, numPoints)
	g := &game{
		buf:        buf,
		titleLayer: ebiten.NewImage(screenW, screenH),
		titlePatch: ebiten.NewImage(shineBandW, shineGradientH),
	}
	g.audioContext = audio.NewContext(audioSampleRate)
	if data, err := os.ReadFile(musicPath); err == nil {
		if stream, err := mp3.DecodeWithSampleRate(audioSampleRate, bytes.NewReader(data)); err == nil {
			if pcm, err := io.ReadAll(stream); err == nil && len(pcm) > 0 {
				g.pcmBuffer = pcm
				if p, err := g.audioContext.NewPlayer(bytes.NewReader(pcm)); err == nil {
					g.musicPlayer = p
				}
			}
		}
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

const bytesPerFrame = 4

func (g *game) _fillBufFromPCM() {
	pos := g.musicPlayer.Position()
	posSec := pos.Seconds()
	currentFrame := int64(posSec * float64(audioSampleRate))
	startFrame := currentFrame - int64(numPoints)
	if startFrame < 0 {
		startFrame = 0
	}
	for i := 0; i < numPoints; i++ {
		frameIdx := startFrame + int64(i)
		byteIdx := frameIdx * bytesPerFrame
		if byteIdx+bytesPerFrame > int64(len(g.pcmBuffer)) {
			g.buf[i] = 0
			continue
		}
		if byteIdx < 0 {
			g.buf[i] = 0
			continue
		}
		lo := binary.LittleEndian.Uint16(g.pcmBuffer[byteIdx : byteIdx+2])
		ro := binary.LittleEndian.Uint16(g.pcmBuffer[byteIdx+2 : byteIdx+4])
		ls := int16(lo)
		rs := int16(ro)
		sample := (float64(ls) + float64(rs)) / 2 / 32768.0
		g.buf[i] = sample
	}
}

func (g *game) _fillBufRandom() {
	g.buf[0] = (rand.Float64() - 0.5) * 0.25
	for i := 1; i < numPoints; i++ {
		delta := (rand.Float64() - 0.5) * 0.08
		g.buf[i] = g.buf[i-1] + delta
	}
}

func (g *game) reset() {
	g.titleFrame = 0
	g.titleSlideT = 0
	g.titleShineT = 0
	g.phase = 0
	g.musicStarted = false
	if g.musicPlayer != nil {
		_ = g.musicPlayer.Rewind()
		g.musicPlayer.Pause()
	}
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
	if len(_konsoleFrames) > 0 {
		g.konsoleFrame++
	}
	if len(_advieFrames) > 0 {
		g.advieFrame++
	}
	oscStart := oscilloscopeStartFrame()
	if g.titleFrame > oscStart+oscilloscopeFlatFrames {
		if len(g.pcmBuffer) > 0 && g.musicPlayer != nil {
			g._fillBufFromPCM()
		} else {
			g._fillBufRandom()
		}
	}
	g.titleFrame++
	if !g.musicStarted && g.titleFrame > titlePhaseTotalFrames() && g.musicPlayer != nil {
		g.musicPlayer.Play()
		g.musicStarted = true
	}
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
	totalTitle := titlePhaseTotalFrames()
	oscStart := oscilloscopeStartFrame()
	if g.titleFrame > oscStart {
		g._drawColumnsAndOscilloscope(screen)
	} else if g.titleFrame > totalTitle {
		g._drawColumnsPhase(screen)
	} else {
		g._drawTitle(screen)
	}
}

var _columnsOffscreen *ebiten.Image

func columnsOffscreen() *ebiten.Image {
	if _columnsOffscreen == nil {
		_columnsOffscreen = ebiten.NewImage(screenW, screenH)
	}
	return _columnsOffscreen
}

func (g *game) _drawColumnsPhase(screen *ebiten.Image) {
	totalTitle := titlePhaseTotalFrames()
	t := float64(g.titleFrame-totalTitle) / float64(columnsPhaseFrames)
	if t > 1 {
		t = 1
	}
	t = t * t * (3 - 2*t)
	zoom := 1.2 - 0.2*t
	off := columnsOffscreen()
	off.Fill(color.Black)
	if _columnLeft != nil && _columnRight != nil {
		lw, lh := _columnLeft.Bounds().Dx(), _columnLeft.Bounds().Dy()
		rw, rh := _columnRight.Bounds().Dx(), _columnRight.Bounds().Dy()
		leftX := float64(-lw) + t*float64(lw)
		rightX := float64(screenW) - t*float64(rw)
		marginY := float64(screenH) * 0.01
		leftY := float64(screenH) - float64(lh) - marginY
		rightY := float64(screenH) - float64(rh) - marginY
		opL := &ebiten.DrawImageOptions{}
		opL.GeoM.Translate(leftX, leftY)
		opL.ColorScale.ScaleAlpha(columnsAlpha)
		off.DrawImage(_columnLeft, opL)
		opR := &ebiten.DrawImageOptions{}
		opR.GeoM.Translate(rightX, rightY)
		opR.ColorScale.ScaleAlpha(columnsAlpha)
		off.DrawImage(_columnRight, opR)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(screenW)/2, -float64(screenH)/2)
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(screenW)/2, float64(screenH)/2)
	screen.DrawImage(off, op)
}

func (g *game) _pulseFromBuf() float64 {
	var sum float64
	for i := 0; i < numPoints; i++ {
		v := g.buf[i]
		sum += v * v
	}
	rms := math.Sqrt(sum / float64(numPoints))
	if rms > 1 {
		rms = 1
	}
	return rms
}

func (g *game) _drawColumnsAndOscilloscope(screen *ebiten.Image) {
	screen.Fill(color.Black)
	if _columnLeft != nil && _columnRight != nil {
		lw, lh := _columnLeft.Bounds().Dx(), _columnLeft.Bounds().Dy()
		rw, rh := _columnRight.Bounds().Dx(), _columnRight.Bounds().Dy()
		marginY := float64(screenH) * 0.01
		leftY := float64(screenH) - float64(lh) - marginY
		rightY := float64(screenH) - float64(rh) - marginY

		pulse := float64(0)
		if g.titleFrame > oscilloscopeStartFrame()+oscilloscopeFlatFrames && len(g.pcmBuffer) > 0 {
			pulse = g._pulseFromBuf()
		}
		s := 1.0 + pulseScale*pulse

		opL := &ebiten.DrawImageOptions{}
		opL.GeoM.Translate(-float64(lw)/2, -float64(lh)/2)
		opL.GeoM.Scale(s, s)
		opL.GeoM.Translate(float64(lw)/2, leftY+float64(lh)/2)
		opL.ColorScale.ScaleAlpha(columnsAlpha)
		screen.DrawImage(_columnLeft, opL)

		opR := &ebiten.DrawImageOptions{}
		opR.GeoM.Translate(-float64(rw)/2, -float64(rh)/2)
		opR.GeoM.Scale(s, s)
		opR.GeoM.Translate(float64(screenW-rw)+float64(rw)/2, rightY+float64(rh)/2)
		opR.ColorScale.ScaleAlpha(columnsAlpha)
		screen.DrawImage(_columnRight, opR)
	}
	g._drawOscilloscope(screen)
	g._drawGIFOverlays(screen)
}

func (g *game) _drawGIFOverlays(screen *ebiten.Image) {
	const frameDelay = 4

	if len(_advieFrames) > 0 {
		f := _advieFrames[(g.advieFrame/frameDelay)%len(_advieFrames)]
		w, _ := f.Bounds().Dx(), f.Bounds().Dy()
		x := float64(screenW-w) / 2
		// górna krawędź 5% poniżej górnej krawędzi okna
		y := float64(screenH) * 0.05
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(x, y)
		screen.DrawImage(f, op)
	}

	if len(_konsoleFrames) > 0 {
		f := _konsoleFrames[(g.konsoleFrame/frameDelay)%len(_konsoleFrames)]
		w, h := f.Bounds().Dx(), f.Bounds().Dy()
		x := float64(screenW-w) / 2
		y := float64(screenH - h)
		op := &ebiten.DrawImageOptions{}
		op.ColorScale.ScaleAlpha(columnsAlpha)
		op.GeoM.Translate(x, y)
		screen.DrawImage(f, op)
	}
}

func (g *game) _drawOscilloscope(screen *ebiten.Image) {
	midY := float64(screenH) / 2
	scaleY := float64(screenH) * 0.35
	lineW := float64(oscilloscopeLineWidth())
	offsetX := float64(oscilloscopeMarginX())
	stepX := lineW / float64(numPoints-1)
	for i := 0; i < numPoints; i++ {
		x := quantize(offsetX+float64(i)*stepX, pixelSize)
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

	g.titleLayer.Fill(color.RGBA{A: 0})
	face := &textv2.GoTextFace{Source: _titleFaceSource, Size: titleFontSize}
	x := titleX
	for i, r := range titleText {
		baseClr := yellowShade(i*5, g.phase)
		baseClr.R = uint8(float64(baseClr.R) * colorBrightness * (1 - fadeT))
		baseClr.G = uint8(float64(baseClr.G) * colorBrightness * (1 - fadeT))
		baseClr.B = uint8(float64(baseClr.B) * colorBrightness * (1 - fadeT))
		if _useV2Text {
			op := &textv2.DrawOptions{}
			op.GeoM.Translate(x, titleY)
			op.ColorScale.ScaleWithColor(baseClr)
			textv2.Draw(g.titleLayer, string(r), face, op)
			w, _ := textv2.Measure(string(r), face, titleFontSize*1.2)
			x += w
		} else {
			textDrawLegacy(g.titleLayer, string(r), int(x), int(titleY), baseClr)
			b := font.MeasureString(basicfont.Face7x13, string(r))
			x += float64(b.Ceil())
		}
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
