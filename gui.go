package main

import (
	"fmt"
	"image/color"
	"strings"
	"sync"

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
	statusLbl *widget.Label
	logLbl    *widget.Label

	mu       sync.Mutex
	logLines []string

	idleColor   color.Color
	activeColor color.Color
}

func NewGUI(cfg Config) *GUI {
	g := &GUI{}

	g.activeColor = guiColors[cfg.PadColorActive]
	if g.activeColor == nil {
		g.activeColor = guiColors["cyan"]
	}
	g.idleColor = guiColors[cfg.PadColorIdle]
	if g.idleColor == nil || cfg.PadColorIdle == "off" {
		g.idleColor = dimColor
	}

	g.app = app.New()
	g.window = g.app.NewWindow("Maschine Mikro MK3 MIDI")
	g.window.Resize(fyne.NewSize(420, 520))
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

			rect := canvas.NewRectangle(g.idleColor)
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

	content := container.NewVBox(
		g.statusLbl,
		widget.NewSeparator(),
		container.NewCenter(grid),
		widget.NewSeparator(),
		g.logLbl,
	)

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
		g.pads[idx].FillColor = g.idleColor
		g.pads[idx].Refresh()
		g.addLogLocked(fmt.Sprintf("Pad %2d OFF", idx+1))
	})
}

func (g *GUI) addLogLocked(line string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.logLines = append(g.logLines, line)
	if len(g.logLines) > 6 {
		g.logLines = g.logLines[len(g.logLines)-6:]
	}
	g.logLbl.SetText(strings.Join(g.logLines, "\n") + "\n")
}

func (g *GUI) Run() {
	g.window.ShowAndRun()
}

func (g *GUI) Quit() {
	g.app.Quit()
}
