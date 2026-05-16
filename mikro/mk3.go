package mikro

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"sync"

	bp "essaim.dev/mikro/api/mk3"
	"github.com/karalabe/hid"
)

const (
	Mk3VID = 0x17cc
	Mk3PID = 0x1700

	buttonReport byte = 1
	padReport    byte = 2
)

type OnPadPunc func(msg PadMessage)
type OnButtonFunc func(msg ButtonMessage)

type Mk3 struct {
	device hid.Device
	path   string

	onPadFunc    OnPadPunc
	onButtonFunc OnButtonFunc

	lights   Lights
	lightsMu sync.RWMutex
	lightsOK bool
}

func OpenMk3() (*Mk3, error) {
	hids, err := hid.Enumerate(Mk3VID, Mk3PID)
	if err != nil {
		return nil, fmt.Errorf("mikro: enumerate: %w", err)
	}
	if len(hids) == 0 {
		return nil, errors.New("mikro: no device found")
	}

	for i, info := range hids {
		log.Printf("mikro: HID[%d] Interface=%d UsagePage=0x%04x Usage=0x%04x Path=%s",
			i, info.Interface, info.UsagePage, info.Usage, info.Path)
	}

	// Select the correct HID interface for pad/button/LED control.
	// The Mikro MK3 exposes multiple USB interfaces; Interface 4 is the
	// control surface (endpoints 0x83 IN / 0x03 OUT).
	selected := 0
	for i, info := range hids {
		if info.Interface == 4 {
			selected = i
			break
		}
	}

	log.Printf("mikro: opening HID[%d] (Interface=%d)", selected, hids[selected].Interface)
	device, err := hids[selected].Open()
	if err != nil {
		return nil, err
	}

	return &Mk3{
		device:   device,
		path:     hids[selected].Path,
		lights:   NewLights(),
		lightsOK: true,
	}, nil
}

func (m *Mk3) Close() error {
	return m.device.Close()
}

func (m *Mk3) SetOnPadFunc(fn OnPadPunc) {
	m.onPadFunc = fn
}

func (m *Mk3) SetOnButtonFunc(fn OnButtonFunc) {
	m.onButtonFunc = fn
}

func (m *Mk3) Lights() Lights {
	m.lightsMu.RLock()
	defer m.lightsMu.RUnlock()

	return m.lights
}

func (m *Mk3) SetLights(lights Lights) error {
	m.lightsMu.Lock()
	if !m.lightsOK {
		m.lights = lights
		m.lightsMu.Unlock()
		return nil
	}

	m.lights = lights
	encoded := m.lights.Encode()

	m.lightsMu.Unlock()

	_, err := writeOutputReport(m.device, m.path, encoded)
	if err != nil {
		m.lightsMu.Lock()
		m.lightsOK = false
		m.lightsMu.Unlock()
		return fmt.Errorf("could not write updated light state: %w", err)
	}

	return nil
}

func (m *Mk3) SetScreen(img image.Image) error {
	i := image.NewPaletted(image.Rect(0, 0, 128, 32), color.Palette{color.Black, color.White})

	draw.FloydSteinberg.Draw(i, i.Bounds(), img, image.Pt(0, 0))

	bitPixels := imageToBit(i)

	stateHigh := bp.ScreenState{
		Magic1:        [3]byte{0xe0, 0x00, 0x00},
		ScreenPortion: 0x00,
		Magic2:        [5]byte{0x00, 0x80, 0x00, 0x02, 0x0},
		Pixels:        [256]byte(bitPixels[:256]),
	}
	stateLow := bp.ScreenState{
		Magic1:        [3]byte{0xe0, 0x00, 0x00},
		ScreenPortion: 0x02,
		Magic2:        [5]byte{0x00, 0x80, 0x00, 0x02, 0x0},
		Pixels:        [256]byte(bitPixels[256:]),
	}

	if _, err := m.device.Write(stateHigh.Encode()); err != nil {
		return fmt.Errorf("could not write updated higher screen state: %w", err)
	}
	if _, err := m.device.Write(stateLow.Encode()); err != nil {
		return fmt.Errorf("could not write updated lower screen state: %w", err)
	}

	return nil
}

func (m *Mk3) Run(ctx context.Context) error {
	b := make([]byte, 1024)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			n, err := m.device.ReadTimeout(b, 500)
			if err != nil {
				return fmt.Errorf("could not read message from device: %w", err)
			}
			if n == 0 {
				continue // timeout, no data yet
			}
			switch b[0] {
			case buttonReport:
				if m.onButtonFunc != nil {
					m.onButtonFunc(m.decodeButtonMessage(b))
				}
			case padReport:
				if m.onPadFunc != nil {
					for _, msg := range m.decodePadMessages(b[:n]) {
						m.onPadFunc(msg)
					}
				}
			default:
				log.Printf("mikro: ignoring unknown report type: 0x%02x", b[0])
			}
		}
	}
}

func (m *Mk3) decodeButtonMessage(buf []byte) ButtonMessage {
	report := bp.ButtonReport{}
	report.Decode(buf)

	return ButtonMessage{
		pressed:         report.PressedButtons,
		encoderPosition: report.EncoderValue,
		encoderTouched:  report.EncoderTouched,
		stripPos1:       report.StripValue1,
		stripPos2:       report.StripValue2,
	}
}

func (m *Mk3) decodePadMessages(buf []byte) []PadMessage {
	msgs := make([]PadMessage, 0, 1)
	for i := 1; i+2 < len(buf); i += 3 {
		pad := buf[i]
		event := buf[i+1] & 0xf0
		velocity := (uint16(buf[i+1]&0x0f) << 8) | uint16(buf[i+2])
		if i > 1 && pad == 0 && event == 0 && velocity == 0 {
			break
		}

		action, ok := decodePadAction(event, velocity)
		if !ok {
			continue
		}
		msgs = append(msgs, PadMessage{
			pad:      Pad(pad),
			velocity: velocity,
			action:   action,
		})
	}

	return msgs
}

func decodePadAction(event byte, velocity uint16) (PadAction, bool) {
	switch event {
	case 0x00, 0x10:
		return PadActionPressed, true
	case 0x20, 0x30:
		return PadActionReleased, true
	case 0x40:
		if velocity == 0 {
			return PadActionReleased, true
		}
		return PadActionTouched, true
	default:
		return 0, false
	}
}

// Converts an image.Image to a byte slice where each pixel is represented by 1 bit
func imageToBit(img *image.Paletted) []byte {
	output := make([]byte, 512)

	for i := range 128 {
		for line := range 4 {
			byteVal := byte(0)
			for bit := range 8 {
				byteVal = byteVal << 1
				if img.At(i, (8*line)+(7-bit)) == color.Black {
					byteVal += 1
				}
			}
			output[128*line+i] = byteVal
		}
	}
	return output
}
