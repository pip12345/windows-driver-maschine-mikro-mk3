package main

import (
	"fmt"
	"os"

	"essaim.dev/mikro"
	"github.com/BurntSushi/toml"
)

type Config struct {
	PortName       string    `toml:"port_name"`
	Channel        uint8     `toml:"channel"`
	PadNotes       [16]uint8 `toml:"pad_notes"`
	PadColorActive string    `toml:"pad_color_active"`
	PadColorIdle   string    `toml:"pad_color_idle"`
}

// Available color names -> mikro.Color
var colorNames = map[string]mikro.Color{
	"off":          mikro.ColorOff,
	"red":          mikro.ColorRed,
	"orange":       mikro.ColorOrange,
	"light_orange": mikro.ColorLightOrange,
	"warm_yellow":  mikro.ColorWarmYellow,
	"yellow":       mikro.ColorYellow,
	"lime":         mikro.ColorLime,
	"green":        mikro.ColorGreen,
	"mint":         mikro.ColorMint,
	"cyan":         mikro.ColorCyan,
	"turquoise":    mikro.ColorTurquoise,
	"blue":         mikro.ColorBlue,
	"plum":         mikro.ColorPlum,
	"violet":       mikro.ColorViolet,
	"purple":       mikro.ColorPurple,
	"magenta":      mikro.ColorMagenta,
	"fuchsia":      mikro.ColorFuchsia,
	"white":        mikro.ColorWhite,
}

func parseColor(name string) (mikro.Color, error) {
	c, ok := colorNames[name]
	if !ok {
		valid := make([]string, 0, len(colorNames))
		for k := range colorNames {
			valid = append(valid, k)
		}
		return 0, fmt.Errorf("unknown color %q (valid: %v)", name, valid)
	}
	return c, nil
}

func defaultConfig() Config {
	return Config{
		PortName:       "Maschine Mikro MK3",
		Channel:        9, // MIDI channel 10 (0-indexed)
		PadNotes:       defaultPadNotes,
		PadColorActive: "cyan",
		PadColorIdle:   "off",
	}
}

func loadConfig(path string) (Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // no config file, use defaults
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Channel > 15 {
		return cfg, fmt.Errorf("channel must be 0-15, got %d", cfg.Channel)
	}
	for i, note := range cfg.PadNotes {
		if note > 127 {
			return cfg, fmt.Errorf("pad_notes[%d] must be 0-127, got %d", i, note)
		}
	}

	if _, err := parseColor(cfg.PadColorActive); err != nil {
		return cfg, fmt.Errorf("pad_color_active: %w", err)
	}
	if _, err := parseColor(cfg.PadColorIdle); err != nil {
		return cfg, fmt.Errorf("pad_color_idle: %w", err)
	}

	return cfg, nil
}
