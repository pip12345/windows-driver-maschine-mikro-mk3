package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"essaim.dev/mikro"
)

func main() {
	log.SetFlags(0)

	cfg, err := loadConfig("config.toml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	activeColor, _ := parseColor(cfg.PadColorActive)
	padIdleColors := parsePadColors(cfg)
	buttonLEDs := parseButtonLEDs(cfg)

	gui := NewGUI(cfg)
	gui.SetStatus("Initializing...")

	go func() {
		// Initialize virtualMIDI DLL
		if err := initVirtualMIDI(); err != nil {
			gui.SetStatus("ERROR: " + err.Error())
			return
		}

		// Create virtual MIDI port
		midiPort, err := CreateVirtualMIDIPort(cfg.PortName)
		if err != nil {
			gui.SetStatus("MIDI port error: " + err.Error())
			return
		}
		defer midiPort.Close()

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		for ctx.Err() == nil {
			mk3, err := mikro.OpenMk3()
			if err != nil {
				gui.SetStatus("MK3 not found: " + err.Error() + "\n\nPlug in your Maschine Mikro MK3. Retrying...")
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
					continue
				}
			}

			gui.SetStatus("Connected - MIDI port: " + cfg.PortName)
			if err := setDefaultLights(mk3, padIdleColors, buttonLEDs); err != nil {
				log.Printf("SetLights error: %v", err)
			}
			var activePads [16]bool
			var activeButtons [40]bool
			var prevEncoder uint8
			var prevStrip1 uint8
			var prevStrip2 uint8
			seenControls := false

			mk3.SetOnPadFunc(func(msg mikro.PadMessage) {
				idx, ok := padIndex[msg.Pad()]
				if !ok {
					return
				}
				note := cfg.PadNotes[idx]

				switch msg.Action() {
				case mikro.PadActionPressed:
					vel := scaleVelocity(msg.Velocity())
					data := noteOn(cfg.Channel, note, vel)
					if err := midiPort.SendData(data); err != nil {
						return
					}
					activePads[idx] = true

					gui.PadOn(idx, vel)

					lights := mk3.Lights()
					lights.Pads[idx] = mikro.ColoredLight{
						Level: mikro.ColorLevelHigh,
						Color: activeColor,
					}
					if err := mk3.SetLights(lights); err != nil {
						log.Printf("SetLights error: %v", err)
					}

				case mikro.PadActionReleased:
					data := noteOff(cfg.Channel, note)
					if err := midiPort.SendData(data); err != nil {
						return
					}
					activePads[idx] = false

					gui.PadOff(idx)

					lights := mk3.Lights()
					lights.Pads[idx] = mikro.ColoredLight{
						Level: mikro.ColorLevelLow,
						Color: padIdleColors[idx],
					}
					if err := mk3.SetLights(lights); err != nil {
						log.Printf("SetLights error: %v", err)
					}
				}
			})

			mk3.SetOnButtonFunc(func(msg mikro.ButtonMessage) {
				pressed := map[mikro.Button]bool{}
				for _, btn := range msg.PressedButtons() {
					pressed[btn] = true
					idx := int(btn)
					if idx < 0 || idx >= len(activeButtons) || activeButtons[idx] {
						continue
					}
					activeButtons[idx] = true

					if cfg.SendButtonNotes {
						_ = midiPort.SendData(noteOn(cfg.Channel, cfg.ButtonNotes[idx], 127))
					}
					if cfg.SendButtonCCs {
						_ = midiPort.SendData(controlChange(cfg.Channel, cfg.ButtonCCs[idx], 127))
					}
					if cfg.EnableTransport {
						switch btn {
						case mikro.ButtonPlay:
							_ = midiPort.SendData(transportStart())
						case mikro.ButtonRestart:
							_ = midiPort.SendData(transportContinue())
						case mikro.ButtonStop:
							_ = midiPort.SendData(transportStop())
						}
					}
					gui.ButtonOn(btn, cfg.ButtonNotes[idx], cfg.ButtonCCs[idx])

					if cfg.ButtonLEDEnabled && idx < 39 {
						lights := mk3.Lights()
						lights.Buttons[idx] = mikro.IntensityHigh
						if err := mk3.SetLights(lights); err != nil {
							log.Printf("SetLights error: %v", err)
						}
					}
				}

				for idx, active := range activeButtons {
					btn := mikro.Button(idx)
					if !active || pressed[btn] {
						continue
					}
					activeButtons[idx] = false
					if cfg.SendButtonNotes {
						_ = midiPort.SendData(noteOff(cfg.Channel, cfg.ButtonNotes[idx]))
					}
					if cfg.SendButtonCCs {
						_ = midiPort.SendData(controlChange(cfg.Channel, cfg.ButtonCCs[idx], 0))
					}
					gui.ButtonOff(btn, cfg.ButtonNotes[idx], cfg.ButtonCCs[idx])
					if cfg.ButtonLEDEnabled && idx < 39 {
						lights := mk3.Lights()
						lights.Buttons[idx] = buttonLEDs[idx]
						if err := mk3.SetLights(lights); err != nil {
							log.Printf("SetLights error: %v", err)
						}
					}
				}

				encoder := msg.EncoderPosition()
				strip1 := msg.StripPosition()
				strip2 := msg.StripSecondPosition()
				if !seenControls {
					prevEncoder = encoder
					prevStrip1 = strip1
					prevStrip2 = strip2
					seenControls = true
					return
				}

				if encoder != prevEncoder {
					delta := int(int8(encoder - prevEncoder))
					if delta != 0 {
						delta = clampEncoderDelta(delta)
						value := relativeCCValue(delta)
						_ = midiPort.SendData(controlChange(cfg.Channel, cfg.EncoderCC, value))
						gui.EncoderTurn(cfg.EncoderCC, value, delta)
					}
					prevEncoder = encoder
				}
				if strip1 != prevStrip1 {
					value := scaleTouchStripValue(strip1, cfg.TouchStripMin, cfg.TouchStripMax)
					send := true
					if strip1 == 0 && cfg.TouchStripRelease == "hold" {
						send = false
					}
					if strip1 == 0 && cfg.TouchStripRelease == "center" {
						value = 64
					}
					if send {
						_ = midiPort.SendData(controlChange(cfg.Channel, cfg.TouchStripCC, value))
						if err := setTouchStripLights(mk3, value, activeColor); err != nil {
							log.Printf("SetLights error: %v", err)
						}
						gui.TouchStrip(cfg.TouchStripCC, value, 1)
					}
					prevStrip1 = strip1
				}
				if strip2 != prevStrip2 {
					value := scaleTouchStripValue(strip2, cfg.TouchStripMin, cfg.TouchStripMax)
					send := true
					if strip2 == 0 && cfg.TouchStripRelease == "hold" {
						send = false
					}
					if strip2 == 0 && cfg.TouchStripRelease == "center" {
						value = 64
					}
					if send {
						_ = midiPort.SendData(controlChange(cfg.Channel, cfg.TouchStripCC2, value))
						if err := setTouchStripLights(mk3, value, activeColor); err != nil {
							log.Printf("SetLights error: %v", err)
						}
						gui.TouchStrip(cfg.TouchStripCC2, value, 2)
					}
					prevStrip2 = strip2
				}
			})

			if err := mk3.Run(ctx); err != nil && ctx.Err() == nil {
				log.Printf("Run error: %v", err)
			}
			if err := mk3.Close(); err != nil {
				log.Printf("Close error: %v", err)
			}
			for idx, active := range activePads {
				if active {
					_ = midiPort.SendData(noteOff(cfg.Channel, cfg.PadNotes[idx]))
					gui.PadOff(idx)
				}
			}
			for idx, active := range activeButtons {
				if active {
					if cfg.SendButtonNotes {
						_ = midiPort.SendData(noteOff(cfg.Channel, cfg.ButtonNotes[idx]))
					}
					if cfg.SendButtonCCs {
						_ = midiPort.SendData(controlChange(cfg.Channel, cfg.ButtonCCs[idx], 0))
					}
				}
			}
			if ctx.Err() == nil {
				gui.SetStatus("MK3 disconnected. Waiting for it to come back...")
			}
		}
	}()

	gui.Run()
}

func setTouchStripLights(mk3 *mikro.Mk3, value uint8, color mikro.Color) error {
	lights := mk3.Lights()
	lit := int((uint16(value)*uint16(len(lights.Strip)) + 63) / 127)
	for idx := range lights.Strip {
		lights.Strip[idx] = mikro.ColoredLight{Color: mikro.ColorOff}
		if idx < lit {
			lights.Strip[idx] = mikro.ColoredLight{
				Level: mikro.ColorLevelHigh,
				Color: color,
			}
		}
	}
	return mk3.SetLights(lights)
}

func parsePadColors(cfg Config) [16]mikro.Color {
	colors := [16]mikro.Color{}
	for idx, name := range cfg.PadColors {
		colors[idx], _ = parseColor(name)
	}
	return colors
}

func parseButtonLEDs(cfg Config) [40]mikro.Intensity {
	leds := [40]mikro.Intensity{}
	for idx, name := range cfg.ButtonLEDs {
		leds[idx], _ = parseIntensity(name)
	}
	return leds
}

func setDefaultLights(mk3 *mikro.Mk3, padColors [16]mikro.Color, buttonLEDs [40]mikro.Intensity) error {
	lights := mk3.Lights()
	for idx, color := range padColors {
		lights.Pads[idx] = mikro.ColoredLight{
			Level: mikro.ColorLevelLow,
			Color: color,
		}
	}
	for idx := range lights.Buttons {
		lights.Buttons[idx] = buttonLEDs[idx]
	}
	return mk3.SetLights(lights)
}
