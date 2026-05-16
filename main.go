package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fynedialog "fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	sqdialog "github.com/sqweek/dialog"
)

// ── Embedded assets ───────────────────────────

//go:embed assets/template.png
var templateBytes []byte

//go:embed assets/outline.png
var outlineBytes []byte

//go:embed assets/icon.png
var iconBytes []byte

//go:embed assets/roboto.ttf
var fontBytes []byte

var fontResource = fyne.NewStaticResource("roboto.ttf", fontBytes)

// Default white color
var warmWhite = color.NRGBA{R: 248, G: 246, B: 235, A: 255}

// Coordinate map

type coord struct {
	imageType               string
	sourceX, sourceY        int
	offsetX, offsetY, width int
}

var sideDrawingCoordinates = map[color.RGBA][]coord{
	{R: 255, G: 0, B: 0, A: 255}: {
		{imageType: "pants", sourceX: 151, sourceY: 355, offsetX: 0, offsetY: 128, width: 64},
		{imageType: "shirt", sourceX: 151, sourceY: 355, offsetX: 0, offsetY: 0, width: 64},
	},
	{R: 0, G: 255, B: 0, A: 255}: {
		{imageType: "pants", sourceX: 217, sourceY: 355, offsetX: 64, offsetY: 128, width: 64},
		{imageType: "pants", sourceX: 231, sourceY: 74, offsetX: 64, offsetY: 0, width: 128},
		{imageType: "pants", sourceX: 308, sourceY: 355, offsetX: 128, offsetY: 128, width: 64},
		{imageType: "shirt", sourceX: 217, sourceY: 355, offsetX: 0, offsetY: 0, width: 64},
		{imageType: "shirt", sourceX: 231, sourceY: 74, offsetX: 64, offsetY: 0, width: 128},
		{imageType: "shirt", sourceX: 308, sourceY: 355, offsetX: 192, offsetY: 0, width: 64},
	},
	{R: 0, G: 0, B: 255, A: 255}: {
		{imageType: "pants", sourceX: 440, sourceY: 355, offsetX: 64, offsetY: 128, width: 64},
		{imageType: "pants", sourceX: 427, sourceY: 74, offsetX: 64, offsetY: 0, width: 128},
		{imageType: "pants", sourceX: 85, sourceY: 355, offsetX: 128, offsetY: 128, width: 64},
		{imageType: "shirt", sourceX: 440, sourceY: 355, offsetX: 0, offsetY: 0, width: 64},
		{imageType: "shirt", sourceX: 427, sourceY: 74, offsetX: 64, offsetY: 0, width: 128},
		{imageType: "shirt", sourceX: 85, sourceY: 355, offsetX: 192, offsetY: 0, width: 64},
	},
	{R: 255, G: 255, B: 0, A: 255}: {
		{imageType: "pants", sourceX: 374, sourceY: 355, offsetX: 0, offsetY: 128, width: 64},
		{imageType: "shirt", sourceX: 374, sourceY: 355, offsetX: 0, offsetY: 0, width: 64},
	},
}

// State

type state struct {
	mu         sync.Mutex
	shirt      image.Image
	pants      image.Image
	background image.Image
	template   image.Image
	outline    image.Image
}

// Helpers

func decodePNG(data []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(data))
}

func toRGBA(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, src, b.Min, draw.Src)
	return dst
}

func loadPNGFromFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// Generate function

func generateShowcase(s *state) *image.RGBA {
	s.mu.Lock()
	shirt := s.shirt
	pants := s.pants
	bg := s.background
	tmpl := s.template
	outline := s.outline
	s.mu.Unlock()

	if shirt == nil && pants == nil && bg == nil {
		if outline != nil {
			return toRGBA(outline)
		}
		return image.NewRGBA(image.Rect(0, 0, 700, 450))
	}

	if tmpl == nil {
		return image.NewRGBA(image.Rect(0, 0, 700, 450))
	}

	bounds := tmpl.Bounds()
	canvasImg := image.NewRGBA(bounds)

	var stretchedBg *image.RGBA

	if bg != nil {
		stretchedBg = image.NewRGBA(bounds)
		bgRGBA := toRGBA(bg)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				srcX := bgRGBA.Bounds().Min.X + (x-bounds.Min.X)*bgRGBA.Bounds().Dx()/bounds.Dx()
				srcY := bgRGBA.Bounds().Min.Y + (y-bounds.Min.Y)*bgRGBA.Bounds().Dy()/bounds.Dy()
				pixel := bgRGBA.At(srcX, srcY)
				stretchedBg.Set(x, y, pixel)
				canvasImg.Set(x, y, pixel)
			}
		}
	}

	tmplRGBA := toRGBA(tmpl)
	draw.Draw(canvasImg, bounds, tmplRGBA, bounds.Min, draw.Over)

	clothing := map[string]image.Image{
		"shirt": shirt,
		"pants": pants,
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := canvasImg.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			pixel := color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: 255,
			}
			coords, ok := sideDrawingCoordinates[pixel]
			if !ok {
				continue
			}

			for _, c := range coords {
				for cy := y + c.offsetY; cy < y+c.offsetY+128; cy++ {
					for cx := x + c.offsetX; cx < x+c.offsetX+c.width; cx++ {
						if canvasImg.Bounds().Min.X <= cx && cx < canvasImg.Bounds().Max.X &&
							canvasImg.Bounds().Min.Y <= cy && cy < canvasImg.Bounds().Max.Y {

							if stretchedBg != nil {
								canvasImg.Set(cx, cy, stretchedBg.At(cx, cy))
							} else {
								canvasImg.SetRGBA(cx, cy, color.RGBA{})
							}
						}
					}
				}
			}

			for _, c := range coords {
				img := clothing[c.imageType]
				if img == nil {
					continue
				}
				imgRGBA := toRGBA(img)
				for cy := 0; cy < 128; cy++ {
					for cx := 0; cx < c.width; cx++ {
						src := imgRGBA.At(c.sourceX+cx, c.sourceY+cy)
						dstX := x + c.offsetX + cx
						dstY := y + c.offsetY + cy
						if canvasImg.Bounds().Min.X <= dstX && dstX < canvasImg.Bounds().Max.X &&
							canvasImg.Bounds().Min.Y <= dstY && dstY < canvasImg.Bounds().Max.Y {
							_, _, _, sa := src.RGBA()
							if sa > 0 {
								canvasImg.Set(dstX, dstY, src)
							}
						}
					}
				}
			}
		}
	}

	return canvasImg
}

// Helpers

func makeFilenameText() *canvas.Text {
	t := canvas.NewText("", warmWhite)
	t.TextSize = 14
	t.Alignment = fyne.TextAlignTrailing
	return t
}

func makeTightSeparator() *canvas.Line {
	l := canvas.NewLine(warmWhite)
	l.StrokeWidth = 1
	return l
}

// UI

type FlatButton struct {
	widget.BaseWidget
	text       string
	onTapped   func()
	bg         *canvas.Rectangle
	label      *canvas.Text
	baseColor  color.Color
	hoverColor color.Color
	flashColor color.Color
	isHovered  bool
	anim       *fyne.Animation
}

var _ desktop.Hoverable = (*FlatButton)(nil)

func NewFlatButton(text string, tapped func()) *FlatButton {
	bColor := color.NRGBA{R: 56, G: 61, B: 63, A: 255}
	hColor := color.NRGBA{R: 68, G: 74, B: 76, A: 255}
	fColor := color.NRGBA{R: 80, G: 87, B: 89, A: 255}

	b := &FlatButton{
		text:       text,
		onTapped:   tapped,
		bg:         canvas.NewRectangle(bColor),
		label:      canvas.NewText(text, warmWhite),
		baseColor:  bColor,
		hoverColor: hColor,
		flashColor: fColor,
	}
	b.label.Alignment = fyne.TextAlignCenter
	b.label.TextSize = 16
	b.ExtendBaseWidget(b)
	return b
}

func (b *FlatButton) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewMax(b.bg, container.NewPadded(b.label)))
}

func (b *FlatButton) MouseIn(*desktop.MouseEvent) {
	b.isHovered = true
	b.animateColor(b.hoverColor, time.Millisecond*150)
}

func (b *FlatButton) MouseMoved(*desktop.MouseEvent) {}

func (b *FlatButton) MouseOut() {
	b.isHovered = false
	b.animateColor(b.baseColor, time.Millisecond*150)
}

func (b *FlatButton) Tapped(_ *fyne.PointEvent) {
	if b.anim != nil {
		b.anim.Stop()
	}
	b.bg.FillColor = b.flashColor
	canvas.Refresh(b.bg)

	targetColor := b.baseColor
	if b.isHovered {
		targetColor = b.hoverColor
	}

	b.animateColor(targetColor, time.Millisecond*300)
	if b.onTapped != nil {
		b.onTapped()
	}
}

func (b *FlatButton) animateColor(target color.Color, duration time.Duration) {
	if b.anim != nil {
		b.anim.Stop()
	}
	b.anim = canvas.NewColorRGBAAnimation(b.bg.FillColor, target, duration, func(c color.Color) {
		b.bg.FillColor = c
		canvas.Refresh(b.bg)
	})
	b.anim.Start()
}

type minWidthLayout struct{ width float32 }

func (m *minWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}
func (m *minWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	minH := float32(0)
	for _, o := range objects {
		if h := o.MinSize().Height; h > minH {
			minH = h
		}
	}
	return fyne.NewSize(m.width, minH)
}

type marginLayout struct {
	top, bottom, left, right float32
}

func (m *marginLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(fyne.NewSize(size.Width-m.left-m.right, size.Height-m.top-m.bottom))
		o.Move(fyne.NewPos(m.left, m.top))
	}
}
func (m *marginLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	minW, minH := float32(0), float32(0)
	for _, o := range objects {
		s := o.MinSize()
		if s.Width > minW {
			minW = s.Width
		}
		if s.Height > minH {
			minH = s.Height
		}
	}
	return fyne.NewSize(minW+m.left+m.right, minH+m.top+m.bottom)
}

type tightLayout struct{}

func (t *tightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for _, o := range objects {
		s := o.MinSize()
		o.Resize(fyne.NewSize(size.Width, s.Height))
		o.Move(fyne.NewPos(0, y))
		y += s.Height
	}
}
func (t *tightLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)
	for _, o := range objects {
		s := o.MinSize()
		if s.Width > w {
			w = s.Width
		}
		h += s.Height
	}
	return fyne.NewSize(w, h)
}

type splitLayout struct {
	panelWidth  float32
	borderThick float32
}

func (s *splitLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 3 {
		return
	}
	canvasObj := objects[0]
	borderObj := objects[1]
	panelObj := objects[2]

	panelObj.Resize(fyne.NewSize(s.panelWidth, size.Height))
	panelObj.Move(fyne.NewPos(size.Width-s.panelWidth, 0))

	borderObj.Resize(fyne.NewSize(s.borderThick, size.Height))
	borderObj.Move(fyne.NewPos(size.Width-s.panelWidth-s.borderThick, 0))

	canvasW := size.Width - s.panelWidth - s.borderThick
	if canvasW < 0 {
		canvasW = 0
	}
	canvasObj.Resize(fyne.NewSize(canvasW, size.Height))
	canvasObj.Move(fyne.NewPos(0, 0))
}

func (s *splitLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 3 {
		return fyne.NewSize(0, 0)
	}
	cMin := objects[0].MinSize()
	return fyne.NewSize(cMin.Width+s.borderThick+s.panelWidth, cMin.Height)
}

// Theme

type customTheme struct{}

var _ fyne.Theme = (*customTheme)(nil)

func (m customTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameBackground {
		return color.NRGBA{R: 24, G: 26, B: 27, A: 255}
	}
	if name == theme.ColorNameOverlayBackground {
		return color.NRGBA{R: 35, G: 40, B: 43, A: 255}
	}
	if name == theme.ColorNameForeground || name == theme.ColorNameSeparator {
		return warmWhite
	}
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (m customTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace {
		return theme.DefaultTheme().Font(style)
	}
	return fontResource
}

func (m customTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m customTheme) Size(name fyne.ThemeSizeName) float32 {
	if strings.HasSuffix(string(name), "Radius") {
		return 0
	}
	if name == theme.SizeNameText {
		return 16
	}
	if name == theme.SizeNamePadding {
		return 8
	}
	return theme.DefaultTheme().Size(name)
}

func main() {
	a := app.New()
	a.Settings().SetTheme(&customTheme{})
	w := a.NewWindow("Clothing Showcaseinator")
	w.SetPadded(false)
	w.SetIcon(fyne.NewStaticResource("icon.png", iconBytes))
	w.Resize(fyne.NewSize(900, 560))
	w.SetFixedSize(false)

	s := &state{}
	if tmpl, err := decodePNG(templateBytes); err == nil {
		s.template = tmpl
	}
	if ol, err := decodePNG(outlineBytes); err == nil {
		s.outline = ol
	}
	canvasImg := canvas.NewImageFromImage(generateShowcase(s))
	canvasImg.FillMode = canvas.ImageFillContain
	canvasImg.ScaleMode = canvas.ImageScalePixels
	canvasImg.SetMinSize(fyne.NewSize(500, 320))

	refresh := func() {
		canvasImg.Image = generateShowcase(s)
		canvasImg.Refresh()
	}

	openImage := func(title string, onLoad func(image.Image, string)) {
		go func() {
			path, err := sqdialog.File().Filter("PNG Image", "png").Title(title).Load()
			if err != nil || path == "" {
				return
			}
			img, err := loadPNGFromFile(path)
			if err != nil {
				fynedialog.ShowError(err, w)
				return
			}
			onLoad(img, filepath.Base(path))
		}()
	}

	createItemRow := func(title string, setImg func(image.Image), lbl *canvas.Text) *fyne.Container {
		upload := NewFlatButton("Upload", func() {
			openImage(title, func(img image.Image, name string) {
				setImg(img)
				lbl.Text = name
				canvas.Refresh(lbl)
				refresh()
			})
		})
		clear := NewFlatButton("Clear", func() {
			setImg(nil)
			lbl.Text = ""
			canvas.Refresh(lbl)
			refresh()
		})

		rowTitle := canvas.NewText(title, warmWhite)
		rowTitle.TextSize = 14

		titleRow := container.NewBorder(nil, nil, rowTitle, nil, lbl)
		buttonRow := container.NewGridWithColumns(2, upload, clear)

		innerGap := canvas.NewRectangle(color.Transparent)
		innerGap.SetMinSize(fyne.NewSize(1, 2))

		return container.New(&tightLayout{},
			titleRow,
			innerGap,
			buttonRow,
		)
	}

	shirtLabel, pantsLabel, bgLabel := makeFilenameText(), makeFilenameText(), makeFilenameText()

	setShirt := func(i image.Image) { s.mu.Lock(); s.shirt = i; s.mu.Unlock() }
	setPants := func(i image.Image) { s.mu.Lock(); s.pants = i; s.mu.Unlock() }
	setBg := func(i image.Image) { s.mu.Lock(); s.background = i; s.mu.Unlock() }

	// Drag and drop
	w.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		if len(uris) == 0 {
			return
		}
		uri := uris[0]

		if strings.ToLower(uri.Extension()) != ".png" {
			fynedialog.ShowInformation("Invalid File", "Only PNG images are supported.", w)
			return
		}

		img, err := loadPNGFromFile(uri.Path())
		if err != nil {
			fynedialog.ShowError(err, w)
			return
		}
		name := filepath.Base(uri.Path())

		var d fynedialog.Dialog

		makeApplyBtn := func(title string, setter func(image.Image), targetLabel *canvas.Text) *FlatButton {
			return NewFlatButton(title, func() {
				setter(img)
				targetLabel.Text = name
				canvas.Refresh(targetLabel)
				refresh()
				d.Hide()
			})
		}

		cancelBtn := NewFlatButton("Cancel", func() {
			d.Hide()
		})

		box := container.NewVBox(
			widget.NewLabel(fmt.Sprintf("Where do you want to apply '%s'?", name)),
			widget.NewSeparator(),
			makeApplyBtn("Apply as Shirt", setShirt, shirtLabel),
			makeApplyBtn("Apply as Pants", setPants, pantsLabel),
			makeApplyBtn("Apply as Background", setBg, bgLabel),
			widget.NewSeparator(),
			cancelBtn,
		)

		d = fynedialog.NewCustomWithoutButtons("File Dropped", box, w)
		d.Show()
	})

	// Save
	saveBtn := NewFlatButton("Save", func() {
		go func() {
			path, err := sqdialog.File().
				Filter("PNG Image", "png").
				Title("Save Showcase").
				SetStartFile("showcase.png").
				Save()

			if err != nil || path == "" {
				return
			}
			if !strings.HasSuffix(strings.ToLower(path), ".png") {
				path += ".png"
			}

			f, err := os.Create(path)
			if err != nil {
				fynedialog.ShowError(err, w)
				return
			}
			defer f.Close()

			if err := png.Encode(f, generateShowcase(s)); err != nil {
				fynedialog.ShowError(err, w)
			}
		}()
	})

	// Layout
	mainTitle := canvas.NewText("Clothing Showcaseinator", warmWhite)
	mainTitle.TextSize = 18

	subTitle := canvas.NewText("by mathew1521", warmWhite)
	subTitle.TextSize = 13

	titleGap := canvas.NewRectangle(color.Transparent)
	titleGap.SetMinSize(fyne.NewSize(1, 8))

	settingsTitle := canvas.NewText("Settings", warmWhite)
	settingsTitle.TextSize = 16

	sectionGap := canvas.NewRectangle(color.Transparent)
	sectionGap.SetMinSize(fyne.NewSize(1, 4))

	settings := container.NewVBox(
		container.New(&tightLayout{}, mainTitle, subTitle),
		titleGap,
		container.New(&tightLayout{}, settingsTitle),
		makeTightSeparator(),
		sectionGap,
		createItemRow("Shirt", setShirt, shirtLabel),
		sectionGap,
		makeTightSeparator(),
		sectionGap,
		createItemRow("Pants", setPants, pantsLabel),
		sectionGap,
		makeTightSeparator(),
		sectionGap,
		createItemRow("Background", setBg, bgLabel),
		sectionGap,
		makeTightSeparator(),
		saveBtn,
	)

	leftBorder := canvas.NewRectangle(warmWhite)

	sidePanel := container.New(
		&marginLayout{top: 20, bottom: 20, left: 20, right: 20},
		container.NewVScroll(container.New(&minWidthLayout{width: 240}, settings)),
	)
	paddedCanvas := container.New(&marginLayout{top: 0, bottom: 0, left: 45, right: 60}, canvasImg)

	bgGradient := canvas.NewRadialGradient(
		color.NRGBA{R: 45, G: 50, B: 53, A: 255},
		color.NRGBA{R: 20, G: 22, B: 23, A: 255},
	)

	mainLayout := container.New(
		&splitLayout{panelWidth: 280, borderThick: 1},
		paddedCanvas, leftBorder, sidePanel,
	)

	w.SetContent(container.NewMax(bgGradient, mainLayout))
	w.ShowAndRun()
}
