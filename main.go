package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"essaim.dev/mikro"
)

func main() {
	log.SetFlags(0)

	cfg, err := loadConfig("config.toml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	activeColor, _ := parseColor(cfg.PadColorActive)
	idleColor, _ := parseColor(cfg.PadColorIdle)

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

		// Open Maschine Mikro MK3
		mk3, err := mikro.OpenMk3()
		if err != nil {
			gui.SetStatus("MK3 not found: " + err.Error() + "\n\nPlug in your Maschine Mikro MK3 and restart.")
			return
		}
		defer mk3.Close()

		gui.SetStatus("Connected - MIDI port: " + cfg.PortName)

		// Set up pad callback
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

				gui.PadOff(idx)

				lights := mk3.Lights()
				lights.Pads[idx] = mikro.ColoredLight{
					Level: mikro.ColorLevelLow,
					Color: idleColor,
				}
				if err := mk3.SetLights(lights); err != nil {
					log.Printf("SetLights error: %v", err)
				}
			}
		})

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		if err := mk3.Run(ctx); err != nil && ctx.Err() == nil {
			gui.SetStatus("Device error: " + err.Error())
			log.Printf("Run error: %v", err)
		}
	}()

	gui.Run()
}
