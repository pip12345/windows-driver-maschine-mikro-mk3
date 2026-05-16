package main

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"log"
	"strings"
	"time"

	"essaim.dev/mikro"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func startScreenWorker(ctx context.Context, mk3 *mikro.Mk3) chan string {
	updates := make(chan string, 32)
	go func() {
		var latest string
		var dirty bool
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case text := <-updates:
				latest = text
				dirty = true
			case <-ticker.C:
				if !dirty {
					continue
				}
				dirty = false
				if err := mk3.SetScreen(renderScreenText(latest)); err != nil {
					log.Printf("SetScreen error: %v", err)
				}
			}
		}
	}()
	return updates
}

func queueScreenText(updates chan string, text string) {
	if text == "" {
		return
	}
	select {
	case updates <- text:
	default:
		select {
		case <-updates:
		default:
		}
		select {
		case updates <- text:
		default:
		}
	}
}

func renderScreenText(text string) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 128, 32))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	parts := strings.SplitN(strings.TrimSpace(text), "\n", 2)
	title := ""
	body := ""
	if len(parts) > 0 {
		title = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		body = strings.TrimSpace(parts[1])
	}
	face := basicfont.Face7x13
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Black),
		Face: face,
	}

	if title != "" {
		draw.Draw(img, image.Rect(0, 0, 128, 16), image.NewUniform(color.Black), image.Point{}, draw.Src)
		d.Src = image.NewUniform(color.White)
		drawCenteredText(d, title, 12)
	}
	d.Src = image.NewUniform(color.Black)
	lines := wrapDisplayText(body, 18, 1)
	startY := 27
	for idx, line := range lines {
		drawCenteredText(d, line, startY+idx*14)
	}
	return img
}

func drawCenteredText(d *font.Drawer, line string, y int) {
	width := d.MeasureString(line).Round()
	x := (128 - width) / 2
	if x < 0 {
		x = 0
	}
	d.Dot = fixed.P(x, y)
	d.DrawString(line)
}

func wrapDisplayText(text string, width, maxLines int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return []string{text}
	}
	lines := make([]string, 0, maxLines)
	for _, part := range parts {
		if len(lines) == 0 {
			lines = append(lines, trimDisplayLine(part, width))
			continue
		}
		last := lines[len(lines)-1]
		if len(last)+1+len(part) <= width {
			lines[len(lines)-1] = last + " " + part
			continue
		}
		if len(lines) >= maxLines {
			break
		}
		lines = append(lines, trimDisplayLine(part, width))
	}
	return lines
}

func trimDisplayLine(line string, width int) string {
	if len(line) <= width {
		return line
	}
	if width <= 1 {
		return line[:width]
	}
	return line[:width-1] + "."
}
