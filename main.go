package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"essaim.dev/mikro"
)

type runtimeConfig struct {
	activeColor   mikro.Color
	activeLevel   mikro.ColorLevel
	padIdleColors [16]mikro.Color
	padIdleLevel  mikro.ColorLevel
	buttonLEDs    [40]mikro.Intensity
}

func newRuntimeConfig(cfg Config) runtimeConfig {
	activeColor, _ := parseColor(cfg.PadColorActive)
	activeLevel, _ := parseColorLevel(cfg.PadLevelActive)
	return runtimeConfig{
		activeColor:   activeColor,
		activeLevel:   activeLevel,
		padIdleColors: parsePadColors(cfg),
		padIdleLevel:  parsePadLevelIdle(cfg),
		buttonLEDs:    parseButtonLEDs(cfg),
	}
}

func main() {
	log.SetFlags(0)

	cfg, err := loadConfig("config.toml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	var currentMk3Mu sync.Mutex
	var currentMk3 *mikro.Mk3
	var currentLightsMu sync.Mutex
	var currentLights chan func(mikro.Lights) mikro.Lights
	var runtimeMu sync.RWMutex
	runtime := newRuntimeConfig(cfg)

	gui := NewGUI(&cfg)
	gui.onConfigSaved = func(updated Config) {
		runtimeMu.Lock()
		runtime = newRuntimeConfig(updated)
		runtimeMu.Unlock()

		currentLightsMu.Lock()
		lights := currentLights
		currentLightsMu.Unlock()
		if lights == nil {
			return
		}
		queueLightUpdate(lights, func(l mikro.Lights) mikro.Lights {
			return defaultLights(l, updated, parsePadColors(updated), parseButtonLEDs(updated))
		})
	}
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
			currentMk3Mu.Lock()
			currentMk3 = mk3
			currentMk3Mu.Unlock()
			deviceCtx, deviceCancel := context.WithCancel(ctx)
			lightUpdates := startLightWorker(deviceCtx, mk3)
			go func() {
				<-deviceCtx.Done()
				_ = mk3.Close()
			}()
			currentLightsMu.Lock()
			currentLights = lightUpdates
			currentLightsMu.Unlock()

			gui.SetStatus("Connected - MIDI port: " + cfg.PortName)
			runtimeMu.RLock()
			rt := runtime
			runtimeMu.RUnlock()
			padIdleColors := rt.padIdleColors
			buttonLEDs := rt.buttonLEDs
			queueLightUpdate(lightUpdates, func(l mikro.Lights) mikro.Lights {
				return defaultLights(l, cfg, padIdleColors, buttonLEDs)
			})
			go blinkButton(lightUpdates, mikro.ButtonProject, buttonLEDs[int(mikro.ButtonProject)], 3)
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
					if activePads[idx] {
						return
					}
					runtimeMu.RLock()
					rt := runtime
					runtimeMu.RUnlock()
					vel := scaleVelocity(msg.Velocity())
					data := noteOn(cfg.Channel, note, vel)
					if err := midiPort.SendData(data); err != nil {
						return
					}
					activePads[idx] = true

					gui.PadOn(idx, vel)

					queueLightUpdate(lightUpdates, func(lights mikro.Lights) mikro.Lights {
						lights.Pads[idx] = mikro.ColoredLight{
							Level: rt.activeLevel,
							Color: rt.activeColor,
						}
						return lights
					})

				case mikro.PadActionReleased:
					if !activePads[idx] {
						return
					}
					data := noteOff(cfg.Channel, note)
					if err := midiPort.SendData(data); err != nil {
						return
					}
					activePads[idx] = false

					gui.PadOff(idx)

					runtimeMu.RLock()
					rt := runtime
					runtimeMu.RUnlock()
					idleLevel := rt.padIdleLevel
					idleColor := rt.padIdleColors[idx]
					queueLightUpdate(lightUpdates, func(lights mikro.Lights) mikro.Lights {
						lights.Pads[idx] = mikro.ColoredLight{
							Level: idleLevel,
							Color: idleColor,
						}
						return lights
					})
				}
			})

			mk3.SetOnButtonFunc(func(msg mikro.ButtonMessage) {
				var pressed [40]bool
				for _, btn := range msg.PressedButtons() {
					idx := int(btn)
					if idx < 0 || idx >= len(pressed) {
						continue
					}
					pressed[idx] = true
					if activeButtons[idx] {
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
						queueLightUpdate(lightUpdates, func(lights mikro.Lights) mikro.Lights {
							lights.Buttons[idx] = mikro.IntensityHigh
							return lights
						})
					}
				}

				for idx, active := range activeButtons {
					btn := mikro.Button(idx)
					if !active || pressed[idx] {
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
						runtimeMu.RLock()
						rt := runtime
						runtimeMu.RUnlock()
						idle := rt.buttonLEDs[idx]
						queueLightUpdate(lightUpdates, func(lights mikro.Lights) mikro.Lights {
							lights.Buttons[idx] = idle
							return lights
						})
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
					runtimeMu.RLock()
					rt := runtime
					runtimeMu.RUnlock()
					value := scaleTouchStripValue(strip1, cfg.TouchStripMin, cfg.TouchStripMax, cfg.TouchStripDeadzone)
					send := true
					if strip1 == 0 && cfg.TouchStripRelease == "hold" {
						send = false
					}
					if strip1 == 0 && cfg.TouchStripRelease == "center" {
						value = 64
					}
					if send {
						_ = midiPort.SendData(controlChange(cfg.Channel, cfg.TouchStripCC, value))
						queueLightUpdate(lightUpdates, func(lights mikro.Lights) mikro.Lights {
							return touchStripLights(lights, value, rt.activeColor)
						})
						gui.TouchStrip(cfg.TouchStripCC, value, 1)
					}
					prevStrip1 = strip1
				}
				if strip2 != prevStrip2 {
					runtimeMu.RLock()
					rt := runtime
					runtimeMu.RUnlock()
					value := scaleTouchStripValue(strip2, cfg.TouchStripMin, cfg.TouchStripMax, cfg.TouchStripDeadzone)
					send := true
					if strip2 == 0 && cfg.TouchStripRelease == "hold" {
						send = false
					}
					if strip2 == 0 && cfg.TouchStripRelease == "center" {
						value = 64
					}
					if send {
						_ = midiPort.SendData(controlChange(cfg.Channel, cfg.TouchStripCC2, value))
						queueLightUpdate(lightUpdates, func(lights mikro.Lights) mikro.Lights {
							return touchStripLights(lights, value, rt.activeColor)
						})
						gui.TouchStrip(cfg.TouchStripCC2, value, 2)
					}
					prevStrip2 = strip2
				}
			})

			if err := mk3.Run(ctx); err != nil && ctx.Err() == nil {
				log.Printf("Run error: %v", err)
			}
			deviceCancel()
			if err := mk3.Close(); err != nil {
				log.Printf("Close error: %v", err)
			}
			currentMk3Mu.Lock()
			if currentMk3 == mk3 {
				currentMk3 = nil
			}
			currentMk3Mu.Unlock()
			currentLightsMu.Lock()
			if currentLights == lightUpdates {
				currentLights = nil
			}
			currentLightsMu.Unlock()
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

func touchStripLights(lights mikro.Lights, value uint8, color mikro.Color) mikro.Lights {
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
	return lights
}

func startLightWorker(ctx context.Context, mk3 *mikro.Mk3) chan func(mikro.Lights) mikro.Lights {
	updates := make(chan func(mikro.Lights) mikro.Lights, 128)
	go func() {
		lights := mk3.Lights()
		dirty := false
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case update := <-updates:
				lights = update(lights)
				dirty = true
			case <-ticker.C:
				if !dirty {
					continue
				}
				dirty = false
				if err := mk3.SetLights(lights); err != nil {
					log.Printf("SetLights error: %v", err)
				}
			}
		}
	}()
	return updates
}

func queueLightUpdate(updates chan func(mikro.Lights) mikro.Lights, update func(mikro.Lights) mikro.Lights) {
	select {
	case updates <- update:
	default:
		// Drop LED-only updates rather than delaying MIDI output during bursts.
	}
}

func parsePadColors(cfg Config) [16]mikro.Color {
	colors := [16]mikro.Color{}
	for idx, name := range cfg.PadColors {
		colors[idx], _ = parseColor(name)
	}
	return colors
}

func parsePadColor(cfg Config, idx int) mikro.Color {
	color, _ := parseColor(cfg.PadColors[idx])
	return color
}

func parsePadLevelIdle(cfg Config) mikro.ColorLevel {
	level, _ := parseColorLevel(cfg.PadLevelIdle)
	return level
}

func parseButtonLEDs(cfg Config) [40]mikro.Intensity {
	leds := [40]mikro.Intensity{}
	for idx, name := range cfg.ButtonLEDs {
		leds[idx], _ = parseIntensity(name)
	}
	return leds
}

func parseButtonLED(cfg Config, idx int) mikro.Intensity {
	led, _ := parseIntensity(cfg.ButtonLEDs[idx])
	return led
}

func defaultLights(lights mikro.Lights, cfg Config, padColors [16]mikro.Color, buttonLEDs [40]mikro.Intensity) mikro.Lights {
	idleLevel := parsePadLevelIdle(cfg)
	for idx, color := range padColors {
		lights.Pads[idx] = mikro.ColoredLight{
			Level: idleLevel,
			Color: color,
		}
	}
	for idx := range lights.Buttons {
		lights.Buttons[idx] = buttonLEDs[idx]
	}
	return lights
}

func blinkButton(updates chan func(mikro.Lights) mikro.Lights, btn mikro.Button, idle mikro.Intensity, count int) {
	idx := int(btn)
	if idx < 0 || idx >= 39 {
		return
	}
	for i := 0; i < count; i++ {
		queueLightUpdate(updates, func(lights mikro.Lights) mikro.Lights {
			lights.Buttons[idx] = mikro.IntensityHigh
			return lights
		})
		time.Sleep(150 * time.Millisecond)

		queueLightUpdate(updates, func(lights mikro.Lights) mikro.Lights {
			lights.Buttons[idx] = idle
			return lights
		})
		time.Sleep(150 * time.Millisecond)
	}
}
