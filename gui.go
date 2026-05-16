package main

import (
	"fmt"
	"image/color"
	"strings"
	"sync"

	"essaim.dev/mikro"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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

	encoderLbl *widget.Label
	stripLbl   *widget.Label
	statusLbl  *widget.Label
	logLbl     *widget.Label

	mu       sync.Mutex
	logLines []string

	padIdleColors [16]color.Color
	activeColor   color.Color
}

var controlIdleColor = color.NRGBA{R: 45, G: 45, B: 50, A: 255}
var controlActiveColor = color.NRGBA{R: 0, G: 120, B: 255, A: 255}

func NewGUI(cfg Config) *GUI {
	g := &GUI{}

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

			rect := canvas.NewRectangle(g.padIdleColors[idx])
			rect.SetMinSize(fyne.NewSize(80, 80))
			rect.CornerRadius = 8

			label := canvas.NewText(fmt.Sprintf("%d (%d)", idx+1, cfg.PadNotes[idx]), color.White)
			label.Alignment = fyne.TextAlignCenter
			label.TextSize = 12

			padContainer := container.NewStack(rect, container.NewCenter(label))
			grid.Add(padContainer)

			g.pads[idx] = rect
			g.padLabels[idx] = label
		}
	}

	buttonGrid := container.NewGridWithColumns(4)
	for idx := 0; idx < len(g.buttons); idx++ {
		rect := canvas.NewRectangle(controlIdleColor)
		rect.SetMinSize(fyne.NewSize(120, 30))
		rect.CornerRadius = 5

		label := canvas.NewText(fmt.Sprintf("%s N%d CC%d", mikro.Button(idx), cfg.ButtonNotes[idx], cfg.ButtonCCs[idx]), color.White)
		label.Alignment = fyne.TextAlignCenter
		label.TextSize = 10

		buttonGrid.Add(container.NewStack(rect, container.NewCenter(label)))
		g.buttons[idx] = rect
	}

	g.encoderLbl = widget.NewLabel(fmt.Sprintf("Encoder: CC%d", cfg.EncoderCC))
	g.stripLbl = widget.NewLabel(fmt.Sprintf("Touch strip: CC%d / CC%d", cfg.TouchStripCC, cfg.TouchStripCC2))

	top := container.NewVBox(
		g.statusLbl,
		widget.NewSeparator(),
		container.NewCenter(container.NewHBox(buttonGrid, grid)),
		widget.NewSeparator(),
		container.NewGridWithColumns(2, g.encoderLbl, g.stripLbl),
		widget.NewSeparator(),
	)
	content := container.NewBorder(top, nil, nil, nil, container.NewVScroll(g.logLbl))

	g.window.SetContent(content)
	return g
}

func (g *GUI) SetStatus(text string) {
	fyne.Do(func() {
		g.statusLbl.SetText(text)
	})
}

func (g *GUI) PadOn(idx int, velocity uint8) {
	if idx < 0 || idx > 15 {
		return
	}
	fyne.Do(func() {
		g.pads[idx].FillColor = g.activeColor
		g.pads[idx].Refresh()
		g.addLogLocked(fmt.Sprintf("Pad %2d ON  vel %3d", idx+1, velocity))
	})
}

func (g *GUI) PadOff(idx int) {
	if idx < 0 || idx > 15 {
		return
	}
	fyne.Do(func() {
		g.pads[idx].FillColor = g.padIdleColors[idx]
		g.pads[idx].Refresh()
		g.addLogLocked(fmt.Sprintf("Pad %2d OFF", idx+1))
	})
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
		g.buttons[idx].FillColor = controlIdleColor
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

func (g *GUI) Run() {
	g.window.ShowAndRun()
}

func (g *GUI) Quit() {
	g.app.Quit()
}
