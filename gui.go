package main

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"sync"
	"time"

	"essaim.dev/mikro"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// Pad colors for the GUI (matching mikro color names to RGB)
var guiColors = map[string]color.Color{
	"off":          color.NRGBA{R: 40, G: 40, B: 40, A: 255},
	"red":          color.NRGBA{R: 255, G: 0, B: 0, A: 255},
	"orange":       color.NRGBA{R: 255, G: 140, B: 0, A: 255},
	"light_orange": color.NRGBA{R: 255, G: 180, B: 50, A: 255},
	"warm_yellow":  color.NRGBA{R: 255, G: 210, B: 60, A: 255},
	"yellow":       color.NRGBA{R: 255, G: 255, B: 0, A: 255},
	"lime":         color.NRGBA{R: 128, G: 255, B: 0, A: 255},
	"green":        color.NRGBA{R: 0, G: 200, B: 0, A: 255},
	"mint":         color.NRGBA{R: 0, G: 220, B: 130, A: 255},
	"cyan":         color.NRGBA{R: 0, G: 255, B: 255, A: 255},
	"turquoise":    color.NRGBA{R: 0, G: 200, B: 200, A: 255},
	"blue":         color.NRGBA{R: 0, G: 80, B: 255, A: 255},
	"plum":         color.NRGBA{R: 140, G: 60, B: 180, A: 255},
	"violet":       color.NRGBA{R: 120, G: 0, B: 255, A: 255},
	"purple":       color.NRGBA{R: 180, G: 0, B: 255, A: 255},
	"magenta":      color.NRGBA{R: 255, G: 0, B: 200, A: 255},
	"fuchsia":      color.NRGBA{R: 255, G: 0, B: 128, A: 255},
	"white":        color.NRGBA{R: 255, G: 255, B: 255, A: 255},
}

var dimColor = color.NRGBA{R: 30, G: 30, B: 30, A: 255}

type GUI struct {
	app    fyne.App
	window fyne.Window

	pads      [16]*canvas.Rectangle
	padLabels [16]*canvas.Text
	buttons   [40]*canvas.Rectangle
	buttonLbl [40]*canvas.Text

	cfg           *Config
	configPath    string
	onConfigSaved func(Config)

	encoderLbl *widget.Label
	stripLbl   *widget.Label
	statusLbl  *widget.Label
	logLbl     *widget.Label

	mu         sync.Mutex
	logLines   []string
	padUpdates chan padUpdate
	logUpdates chan string

	padIdleColors [16]color.Color
	activeColor   color.Color
}

type padUpdate struct {
	idx      int
	active   bool
	velocity uint8
}

var controlIdleColor = color.NRGBA{R: 45, G: 45, B: 50, A: 255}
var controlLowColor = color.NRGBA{R: 55, G: 65, B: 80, A: 255}
var controlMediumColor = color.NRGBA{R: 65, G: 90, B: 120, A: 255}
var controlHighColor = color.NRGBA{R: 75, G: 115, B: 165, A: 255}
var controlActiveColor = color.NRGBA{R: 0, G: 120, B: 255, A: 255}

type tappableStack struct {
	widget.BaseWidget
	onTapped          func()
	onSecondaryTapped func(*fyne.PointEvent)
	objects           []fyne.CanvasObject
}

func newTappableStack(onTapped func(), onSecondaryTapped func(*fyne.PointEvent), objects ...fyne.CanvasObject) *tappableStack {
	t := &tappableStack{onTapped: onTapped, onSecondaryTapped: onSecondaryTapped, objects: objects}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableStack) Tapped(*fyne.PointEvent) {
	if t.onTapped != nil {
		t.onTapped()
	}
}

func (t *tappableStack) TappedSecondary(e *fyne.PointEvent) {
	if t.onSecondaryTapped != nil {
		t.onSecondaryTapped(e)
	}
}

func (t *tappableStack) CreateRenderer() fyne.WidgetRenderer {
	return &tappableStackRenderer{objects: t.objects}
}

type tappableStackRenderer struct {
	objects []fyne.CanvasObject
}

func (r *tappableStackRenderer) Layout(size fyne.Size) {
	for _, obj := range r.objects {
		obj.Resize(size)
	}
}

func (r *tappableStackRenderer) MinSize() fyne.Size {
	min := fyne.NewSize(0, 0)
	for _, obj := range r.objects {
		objMin := obj.MinSize()
		if objMin.Width > min.Width {
			min.Width = objMin.Width
		}
		if objMin.Height > min.Height {
			min.Height = objMin.Height
		}
	}
	return min
}

func (r *tappableStackRenderer) Refresh() {
	for _, obj := range r.objects {
		obj.Refresh()
	}
}

func (r *tappableStackRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *tappableStackRenderer) Destroy() {}

func NewGUI(cfg *Config) *GUI {
	g := &GUI{
		cfg:        cfg,
		configPath: "config.toml",
		padUpdates: make(chan padUpdate, 128),
		logUpdates: make(chan string, 1024),
	}

	g.activeColor = guiColors[cfg.PadColorActive]
	if g.activeColor == nil {
		g.activeColor = guiColors["cyan"]
	}
	for idx, padColor := range cfg.PadColors {
		g.padIdleColors[idx] = guiColors[padColor]
		if g.padIdleColors[idx] == nil || padColor == "off" {
			g.padIdleColors[idx] = dimColor
		}
	}

	g.app = app.New()
	g.window = g.app.NewWindow("Maschine Mikro MK3 MIDI")
	g.window.Resize(fyne.NewSize(900, 780))
	g.window.SetFixedSize(true)

	g.statusLbl = widget.NewLabel("Starting...")
	g.statusLbl.Wrapping = fyne.TextWrapWord

	g.logLbl = widget.NewLabel("")
	g.logLbl.Wrapping = fyne.TextWrapWord
	g.logLbl.TextStyle = fyne.TextStyle{Monospace: true}

	// Build 4x4 pad grid (row 4 on top, row 1 on bottom, matching physical layout)
	grid := container.NewGridWithColumns(4)
	for row := 3; row >= 0; row-- {
		for col := 0; col < 4; col++ {
			idx := row*4 + col
			padIdx := idx

			rect := canvas.NewRectangle(g.padIdleColors[idx])
			rect.SetMinSize(fyne.NewSize(80, 80))
			rect.CornerRadius = 8

			label := canvas.NewText(g.padLabel(padIdx), color.White)
			label.Alignment = fyne.TextAlignCenter
			label.TextStyle = fyne.TextStyle{Bold: true}
			label.TextSize = 12

			padContainer := newTappableStack(
				func() { g.showPadConfig(padIdx) },
				func(e *fyne.PointEvent) { g.showPadColorMenu(padIdx, e.AbsolutePosition) },
				rect,
				container.NewCenter(label),
			)
			grid.Add(padContainer)

			g.pads[padIdx] = rect
			g.padLabels[padIdx] = label
		}
	}

	buttonGrid := container.NewGridWithColumns(4)
	for idx := 0; idx < len(g.buttons); idx++ {
		buttonIdx := idx
		rect := canvas.NewRectangle(g.buttonIdleColor(buttonIdx))
		rect.SetMinSize(fyne.NewSize(120, 30))
		rect.CornerRadius = 5

		label := canvas.NewText(g.buttonLabel(buttonIdx), color.White)
		label.Alignment = fyne.TextAlignCenter
		label.TextStyle = fyne.TextStyle{Bold: true}
		label.TextSize = 10

		buttonGrid.Add(newTappableStack(
			func() { g.showButtonConfig(buttonIdx) },
			func(e *fyne.PointEvent) { g.showButtonLEDMenu(buttonIdx, e.AbsolutePosition) },
			rect,
			container.NewCenter(label),
		))
		g.buttons[buttonIdx] = rect
		g.buttonLbl[buttonIdx] = label
	}

	g.encoderLbl = widget.NewLabel(fmt.Sprintf("Encoder: CC%d", cfg.EncoderCC))
	g.stripLbl = widget.NewLabel(fmt.Sprintf("Touch strip: CC%d / CC%d", cfg.TouchStripCC, cfg.TouchStripCC2))
	configButton := widget.NewButton("Config", g.showGlobalConfig)

	top := container.NewVBox(
		container.NewBorder(nil, nil, nil, configButton, g.statusLbl),
		widget.NewSeparator(),
		container.NewCenter(container.NewHBox(buttonGrid, grid)),
		widget.NewSeparator(),
		container.NewGridWithColumns(2, g.encoderLbl, g.stripLbl),
		widget.NewSeparator(),
	)
	content := container.NewBorder(top, nil, nil, nil, container.NewVScroll(g.logLbl))

	g.window.SetContent(content)
	g.startPadUpdateWorker()
	g.startLogWorker()
	return g
}

func (g *GUI) padLabel(idx int) string {
	return fmt.Sprintf("%d N%d %s", idx+1, g.cfg.PadNotes[idx], g.cfg.PadColors[idx])
}

func (g *GUI) buttonLabel(idx int) string {
	return fmt.Sprintf("%s N%d CC%d", mikro.Button(idx), g.cfg.ButtonNotes[idx], g.cfg.ButtonCCs[idx])
}

func (g *GUI) buttonIdleColor(idx int) color.Color {
	switch g.cfg.ButtonLEDs[idx] {
	case "low":
		return controlLowColor
	case "medium":
		return controlMediumColor
	case "high":
		return controlHighColor
	default:
		return controlIdleColor
	}
}

func (g *GUI) refreshConfigLabels() {
	for idx := range g.padLabels {
		if g.padLabels[idx] != nil {
			g.padLabels[idx].Text = g.padLabel(idx)
			g.padLabels[idx].Refresh()
		}
		g.padIdleColors[idx] = guiColors[g.cfg.PadColors[idx]]
		if g.padIdleColors[idx] == nil || g.cfg.PadColors[idx] == "off" {
			g.padIdleColors[idx] = dimColor
		}
		if g.pads[idx] != nil {
			g.pads[idx].FillColor = g.padIdleColors[idx]
			g.pads[idx].Refresh()
		}
	}
	for idx := range g.buttonLbl {
		if g.buttonLbl[idx] != nil {
			g.buttonLbl[idx].Text = g.buttonLabel(idx)
			g.buttonLbl[idx].Refresh()
		}
		if g.buttons[idx] != nil {
			g.buttons[idx].FillColor = g.buttonIdleColor(idx)
			g.buttons[idx].Refresh()
		}
	}
	g.activeColor = guiColors[g.cfg.PadColorActive]
	if g.activeColor == nil {
		g.activeColor = guiColors["cyan"]
	}
	g.encoderLbl.SetText(fmt.Sprintf("Encoder: CC%d", g.cfg.EncoderCC))
	g.stripLbl.SetText(fmt.Sprintf("Touch strip: CC%d / CC%d", g.cfg.TouchStripCC, g.cfg.TouchStripCC2))
}

func (g *GUI) saveConfig() bool {
	if err := saveConfig(g.configPath, *g.cfg); err != nil {
		dialog.ShowError(err, g.window)
		return false
	}
	g.refreshConfigLabels()
	if g.onConfigSaved != nil {
		g.onConfigSaved(*g.cfg)
	}
	g.addLogLocked("Saved config.toml")
	return true
}

func (g *GUI) showPadConfig(idx int) {
	note := widget.NewEntry()
	note.SetText(strconv.Itoa(int(g.cfg.PadNotes[idx])))
	colorSelect := widget.NewSelect(colorOptions(), nil)
	colorSelect.SetSelected(g.cfg.PadColors[idx])

	d := dialog.NewForm(fmt.Sprintf("Pad %d Config", idx+1), "Save", "Cancel", []*widget.FormItem{
		widget.NewFormItem("MIDI note", note),
		widget.NewFormItem("Idle color", colorSelect),
	}, func(ok bool) {
		if !ok {
			return
		}
		n, err := parseUint8Entry("MIDI note", note.Text, 0, 127)
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}
		g.cfg.PadNotes[idx] = n
		g.cfg.PadColors[idx] = colorSelect.Selected
		g.saveConfig()
	}, g.window)
	d.Show()
}

func (g *GUI) showPadColorMenu(idx int, pos fyne.Position) {
	items := make([]*fyne.MenuItem, 0, len(colorOptions()))
	for _, colorName := range colorOptions() {
		name := colorName
		items = append(items, fyne.NewMenuItem(name, func() {
			g.cfg.PadColors[idx] = name
			g.saveConfig()
		}))
	}
	widget.ShowPopUpMenuAtPosition(fyne.NewMenu("Pad Color", items...), g.window.Canvas(), pos)
}

func (g *GUI) showButtonConfig(idx int) {
	note := widget.NewEntry()
	note.SetText(strconv.Itoa(int(g.cfg.ButtonNotes[idx])))
	cc := widget.NewEntry()
	cc.SetText(strconv.Itoa(int(g.cfg.ButtonCCs[idx])))
	ledSelect := widget.NewSelect(intensityOptions(), nil)
	ledSelect.SetSelected(g.cfg.ButtonLEDs[idx])

	d := dialog.NewForm(fmt.Sprintf("%s Config", mikro.Button(idx)), "Save", "Cancel", []*widget.FormItem{
		widget.NewFormItem("MIDI note", note),
		widget.NewFormItem("MIDI CC", cc),
		widget.NewFormItem("Default LED", ledSelect),
	}, func(ok bool) {
		if !ok {
			return
		}
		n, err := parseUint8Entry("MIDI note", note.Text, 0, 127)
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}
		c, err := parseUint8Entry("MIDI CC", cc.Text, 0, 127)
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}
		g.cfg.ButtonNotes[idx] = n
		g.cfg.ButtonCCs[idx] = c
		g.cfg.ButtonLEDs[idx] = ledSelect.Selected
		g.saveConfig()
	}, g.window)
	d.Show()
}

func (g *GUI) showButtonLEDMenu(idx int, pos fyne.Position) {
	items := make([]*fyne.MenuItem, 0, len(intensityOptions()))
	for _, intensity := range intensityOptions() {
		name := intensity
		items = append(items, fyne.NewMenuItem(name, func() {
			g.cfg.ButtonLEDs[idx] = name
			g.saveConfig()
		}))
	}
	widget.ShowPopUpMenuAtPosition(fyne.NewMenu("Button LED", items...), g.window.Canvas(), pos)
}

func (g *GUI) showGlobalConfig() {
	port := widget.NewEntry()
	port.SetText(g.cfg.PortName)
	channel := widget.NewEntry()
	channel.SetText(strconv.Itoa(int(g.cfg.Channel)))
	activeColor := widget.NewSelect(colorOptions(), nil)
	activeColor.SetSelected(g.cfg.PadColorActive)
	activeLevel := widget.NewSelect(colorLevelOptions(), nil)
	activeLevel.SetSelected(g.cfg.PadLevelActive)
	idleLevel := widget.NewSelect(colorLevelOptions(), nil)
	idleLevel.SetSelected(g.cfg.PadLevelIdle)
	encoderCC := widget.NewEntry()
	encoderCC.SetText(strconv.Itoa(int(g.cfg.EncoderCC)))
	stripCC := widget.NewEntry()
	stripCC.SetText(strconv.Itoa(int(g.cfg.TouchStripCC)))
	stripCC2 := widget.NewEntry()
	stripCC2.SetText(strconv.Itoa(int(g.cfg.TouchStripCC2)))
	stripMin := widget.NewEntry()
	stripMin.SetText(strconv.Itoa(int(g.cfg.TouchStripMin)))
	stripMax := widget.NewEntry()
	stripMax.SetText(strconv.Itoa(int(g.cfg.TouchStripMax)))
	stripDeadzone := widget.NewEntry()
	stripDeadzone.SetText(strconv.Itoa(int(g.cfg.TouchStripDeadzone)))
	stripRelease := widget.NewSelect([]string{"hold", "zero", "center"}, nil)
	stripRelease.SetSelected(g.cfg.TouchStripRelease)
	sendNotes := widget.NewCheck("", nil)
	sendNotes.SetChecked(g.cfg.SendButtonNotes)
	sendCCs := widget.NewCheck("", nil)
	sendCCs.SetChecked(g.cfg.SendButtonCCs)
	transport := widget.NewCheck("", nil)
	transport.SetChecked(g.cfg.EnableTransport)
	buttonLEDs := widget.NewCheck("", nil)
	buttonLEDs.SetChecked(g.cfg.ButtonLEDEnabled)

	d := dialog.NewForm("Global Config", "Save", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Port name", port),
		widget.NewFormItem("MIDI channel", channel),
		widget.NewFormItem("Active pad color", activeColor),
		widget.NewFormItem("Pad active brightness", activeLevel),
		widget.NewFormItem("Pad idle brightness", idleLevel),
		widget.NewFormItem("Encoder CC", encoderCC),
		widget.NewFormItem("Touch strip CC 1", stripCC),
		widget.NewFormItem("Touch strip CC 2", stripCC2),
		widget.NewFormItem("Touch strip min", stripMin),
		widget.NewFormItem("Touch strip max", stripMax),
		widget.NewFormItem("Touch strip deadzone", stripDeadzone),
		widget.NewFormItem("Touch release", stripRelease),
		widget.NewFormItem("Send button notes", sendNotes),
		widget.NewFormItem("Send button CCs", sendCCs),
		widget.NewFormItem("Transport", transport),
		widget.NewFormItem("Button LEDs", buttonLEDs),
	}, func(ok bool) {
		if !ok {
			return
		}
		ch, err := parseUint8Entry("MIDI channel", channel.Text, 0, 15)
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}
		enc, err := parseUint8Entry("Encoder CC", encoderCC.Text, 0, 127)
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}
		s1, err := parseUint8Entry("Touch strip CC 1", stripCC.Text, 0, 127)
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}
		s2, err := parseUint8Entry("Touch strip CC 2", stripCC2.Text, 0, 127)
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}
		min, err := parseUint8Entry("Touch strip min", stripMin.Text, 0, 254)
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}
		max, err := parseUint8Entry("Touch strip max", stripMax.Text, 1, 255)
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}
		if min >= max {
			dialog.ShowError(fmt.Errorf("touch strip min must be less than max"), g.window)
			return
		}
		deadzone, err := parseUint8Entry("Touch strip deadzone", stripDeadzone.Text, 0, int(max-min))
		if err != nil {
			dialog.ShowError(err, g.window)
			return
		}

		g.cfg.PortName = port.Text
		g.cfg.Channel = ch
		g.cfg.PadColorActive = activeColor.Selected
		g.cfg.PadLevelActive = activeLevel.Selected
		g.cfg.PadLevelIdle = idleLevel.Selected
		g.cfg.EncoderCC = enc
		g.cfg.TouchStripCC = s1
		g.cfg.TouchStripCC2 = s2
		g.cfg.TouchStripMin = min
		g.cfg.TouchStripMax = max
		g.cfg.TouchStripDeadzone = deadzone
		g.cfg.TouchStripRelease = stripRelease.Selected
		g.cfg.SendButtonNotes = sendNotes.Checked
		g.cfg.SendButtonCCs = sendCCs.Checked
		g.cfg.EnableTransport = transport.Checked
		g.cfg.ButtonLEDEnabled = buttonLEDs.Checked
		g.saveConfig()
	}, g.window)
	d.Resize(fyne.NewSize(460, 640))
	d.Show()
}

func parseUint8Entry(name, text string, min, max int) (uint8, error) {
	v, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", name)
	}
	if v < min || v > max {
		return 0, fmt.Errorf("%s must be %d-%d", name, min, max)
	}
	return uint8(v), nil
}

func colorOptions() []string {
	return []string{"off", "red", "orange", "light_orange", "warm_yellow", "yellow", "lime", "green", "mint", "cyan", "turquoise", "blue", "plum", "violet", "purple", "magenta", "fuchsia", "white"}
}

func intensityOptions() []string {
	return []string{"off", "low", "medium", "high"}
}

func colorLevelOptions() []string {
	return []string{"low", "medium", "high", "faded"}
}

func (g *GUI) SetStatus(text string) {
	fyne.Do(func() {
		g.statusLbl.SetText(text)
	})
}

func (g *GUI) startPadUpdateWorker() {
	go func() {
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()
		var latest [16]padUpdate
		var pending [16]bool
		for {
			select {
			case update := <-g.padUpdates:
				latest[update.idx] = update
				pending[update.idx] = true
			case <-ticker.C:
				updates := latest
				toApply := pending
				pending = [16]bool{}
				if toApply == [16]bool{} {
					continue
				}
				fyne.Do(func() {
					for idx, ok := range toApply {
						if !ok {
							continue
						}
						update := updates[idx]
						if update.active {
							g.pads[idx].FillColor = g.activeColor
							g.pads[idx].Refresh()
							continue
						}
						g.pads[idx].FillColor = g.padIdleColors[idx]
						g.pads[idx].Refresh()
					}
				})
			}
		}
	}()
}

func (g *GUI) PadOn(idx int, velocity uint8) {
	if idx < 0 || idx > 15 {
		return
	}
	select {
	case g.padUpdates <- padUpdate{idx: idx, active: true, velocity: velocity}:
	default:
	}
	g.addLogAsync(fmt.Sprintf("Pad %2d ON  vel %3d", idx+1, velocity))
}

func (g *GUI) PadOff(idx int) {
	if idx < 0 || idx > 15 {
		return
	}
	select {
	case g.padUpdates <- padUpdate{idx: idx}:
	default:
	}
	g.addLogAsync(fmt.Sprintf("Pad %2d OFF", idx+1))
}

func (g *GUI) ButtonOn(btn mikro.Button, note, cc uint8) {
	idx := int(btn)
	if idx < 0 || idx >= len(g.buttons) {
		return
	}
	fyne.Do(func() {
		g.buttons[idx].FillColor = controlActiveColor
		g.buttons[idx].Refresh()
		g.addLogLocked(fmt.Sprintf("Button %-10s ON  note %3d cc %3d", btn, note, cc))
	})
}

func (g *GUI) ButtonOff(btn mikro.Button, note, cc uint8) {
	idx := int(btn)
	if idx < 0 || idx >= len(g.buttons) {
		return
	}
	fyne.Do(func() {
		g.buttons[idx].FillColor = g.buttonIdleColor(idx)
		g.buttons[idx].Refresh()
		g.addLogLocked(fmt.Sprintf("Button %-10s OFF note %3d cc %3d", btn, note, cc))
	})
}

func (g *GUI) EncoderTurn(cc, value uint8, delta int) {
	fyne.Do(func() {
		g.encoderLbl.SetText(fmt.Sprintf("Encoder: CC%d value %d delta %+d", cc, value, delta))
		g.addLogLocked(fmt.Sprintf("Encoder CC %3d value %3d delta %+d", cc, value, delta))
	})
}

func (g *GUI) TouchStrip(cc, value uint8, strip int) {
	fyne.Do(func() {
		g.stripLbl.SetText(fmt.Sprintf("Touch strip %d: CC%d value %d", strip, cc, value))
		g.addLogLocked(fmt.Sprintf("Strip %d CC %3d value %3d", strip, cc, value))
	})
}

func (g *GUI) addLogLocked(line string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.logLines = append([]string{line}, g.logLines...)
	if len(g.logLines) > 30 {
		g.logLines = g.logLines[:30]
	}
	g.logLbl.SetText(strings.Join(g.logLines, "\n") + "\n")
}

func (g *GUI) addLogAsync(line string) {
	select {
	case g.logUpdates <- line:
	default:
		select {
		case <-g.logUpdates:
		default:
		}
		select {
		case g.logUpdates <- line:
		default:
		}
	}
}

func (g *GUI) startLogWorker() {
	go func() {
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()
		pending := make([]string, 0, 64)
		for {
			select {
			case line := <-g.logUpdates:
				pending = append(pending, line)
			case <-ticker.C:
				if len(pending) == 0 {
					continue
				}
				lines := append([]string(nil), pending...)
				pending = pending[:0]
				fyne.Do(func() {
					g.mu.Lock()
					defer g.mu.Unlock()
					for i := len(lines) - 1; i >= 0; i-- {
						g.logLines = append([]string{lines[i]}, g.logLines...)
					}
					if len(g.logLines) > 30 {
						g.logLines = g.logLines[:30]
					}
					g.logLbl.SetText(strings.Join(g.logLines, "\n") + "\n")
				})
			}
		}
	}()
}

func (g *GUI) Run() {
	g.window.ShowAndRun()
}

func (g *GUI) Quit() {
	g.app.Quit()
}
