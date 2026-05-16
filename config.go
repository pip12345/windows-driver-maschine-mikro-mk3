package main

import (
	"fmt"
	"os"

	"essaim.dev/mikro"
	"github.com/BurntSushi/toml"
)

type Config struct {
	PortName           string     `toml:"port_name"`
	Channel            uint8      `toml:"channel"`
	PadNotes           [16]uint8  `toml:"pad_notes"`
	PadColorActive     string     `toml:"pad_color_active"`
	PadColorIdle       string     `toml:"pad_color_idle"`
	PadLevelActive     string     `toml:"pad_level_active"`
	PadLevelIdle       string     `toml:"pad_level_idle"`
	PadColors          [16]string `toml:"pad_colors"`
	PadLevels          [16]string `toml:"pad_levels"`
	PadDisplayTexts    [16]string `toml:"pad_display_texts"`
	SendButtonNotes    bool       `toml:"send_button_notes"`
	SendButtonCCs      bool       `toml:"send_button_ccs"`
	ButtonNotes        [40]uint8  `toml:"button_notes"`
	ButtonCCs          [40]uint8  `toml:"button_ccs"`
	ButtonLEDIdle      string     `toml:"button_led_idle"`
	ButtonLEDs         [40]string `toml:"button_leds"`
	ButtonDisplayTexts [40]string `toml:"button_display_texts"`
	DisplayMode        string     `toml:"display_mode"`
	DisplayText        string     `toml:"display_text"`
	EncoderCC          uint8      `toml:"encoder_cc"`
	TouchStripCC       uint8      `toml:"touch_strip_cc"`
	TouchStripCC2      uint8      `toml:"touch_strip_cc_2"`
	TouchStripRelease  string     `toml:"touch_strip_release"`
	TouchStripMin      uint8      `toml:"touch_strip_min"`
	TouchStripMax      uint8      `toml:"touch_strip_max"`
	TouchStripDeadzone uint8      `toml:"touch_strip_deadzone"`
	EnableTransport    bool       `toml:"enable_transport"`
	ButtonLEDEnabled   bool       `toml:"button_led_enabled"`
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

var intensityNames = map[string]mikro.Intensity{
	"off":    mikro.IntensityOff,
	"low":    mikro.IntensityLow,
	"medium": mikro.IntensityMedium,
	"high":   mikro.IntensityHigh,
}

var colorLevelNames = map[string]mikro.ColorLevel{
	"low":    mikro.ColorLevelLow,
	"medium": mikro.ColorLevelMedium,
	"high":   mikro.ColorLevelHigh,
	"faded":  mikro.ColorLevelFaded,
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

func parseIntensity(name string) (mikro.Intensity, error) {
	i, ok := intensityNames[name]
	if !ok {
		return 0, fmt.Errorf("unknown intensity %q (valid: off, low, medium, high)", name)
	}
	return i, nil
}

func parseColorLevel(name string) (mikro.ColorLevel, error) {
	level, ok := colorLevelNames[name]
	if !ok {
		return 0, fmt.Errorf("unknown color level %q (valid: low, medium, high, faded)", name)
	}
	return level, nil
}

func defaultPadColors() [16]string {
	colors := [16]string{}
	for i := range colors {
		colors[i] = "off"
	}
	return colors
}

func defaultPadLevels() [16]string {
	levels := [16]string{}
	for i := range levels {
		levels[i] = "low"
	}
	return levels
}

func defaultButtonLEDs() [40]string {
	leds := [40]string{}
	for i := range leds {
		leds[i] = "off"
	}
	return leds
}

func defaultPadDisplayTexts() [16]string {
	texts := [16]string{}
	return texts
}

func defaultButtonDisplayTexts() [40]string {
	texts := [40]string{}
	return texts
}

func defaultConfig() Config {
	return Config{
		PortName:           "Maschine Mikro MK3",
		Channel:            9, // MIDI channel 10 (0-indexed)
		PadNotes:           defaultPadNotes,
		PadColorActive:     "cyan",
		PadColorIdle:       "off",
		PadLevelActive:     "high",
		PadLevelIdle:       "low",
		PadColors:          defaultPadColors(),
		PadLevels:          defaultPadLevels(),
		PadDisplayTexts:    defaultPadDisplayTexts(),
		SendButtonNotes:    true,
		SendButtonCCs:      true,
		ButtonNotes:        defaultButtonNotes,
		ButtonCCs:          defaultButtonCCs,
		ButtonLEDIdle:      "off",
		ButtonLEDs:         defaultButtonLEDs(),
		ButtonDisplayTexts: defaultButtonDisplayTexts(),
		DisplayMode:        "trigger",
		DisplayText:        "Maschine Mikro MK3",
		EncoderCC:          16,
		TouchStripCC:       17,
		TouchStripCC2:      18,
		TouchStripRelease:  "hold",
		TouchStripMin:      0,
		TouchStripMax:      200,
		TouchStripDeadzone: 1,
		EnableTransport:    true,
		ButtonLEDEnabled:   true,
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
	if cfg.PadColorIdle != "off" {
		allPadColorsOff := true
		for _, color := range cfg.PadColors {
			if color != "off" {
				allPadColorsOff = false
				break
			}
		}
		if allPadColorsOff {
			for i := range cfg.PadColors {
				cfg.PadColors[i] = cfg.PadColorIdle
			}
		}
	}
	allPadLevelsDefault := true
	for _, level := range cfg.PadLevels {
		if level != "low" && level != "" {
			allPadLevelsDefault = false
			break
		}
	}
	if allPadLevelsDefault && cfg.PadLevelIdle != "low" {
		for i := range cfg.PadLevels {
			cfg.PadLevels[i] = cfg.PadLevelIdle
		}
	}
	if cfg.ButtonLEDIdle != "off" {
		allButtonLEDsOff := true
		for _, led := range cfg.ButtonLEDs {
			if led != "off" {
				allButtonLEDsOff = false
				break
			}
		}
		if allButtonLEDsOff {
			for i := range cfg.ButtonLEDs {
				cfg.ButtonLEDs[i] = cfg.ButtonLEDIdle
			}
		}
	}

	if cfg.Channel > 15 {
		return cfg, fmt.Errorf("channel must be 0-15, got %d", cfg.Channel)
	}
	for i, note := range cfg.PadNotes {
		if note > 127 {
			return cfg, fmt.Errorf("pad_notes[%d] must be 0-127, got %d", i, note)
		}
	}
	for i, note := range cfg.ButtonNotes {
		if note > 127 {
			return cfg, fmt.Errorf("button_notes[%d] must be 0-127, got %d", i, note)
		}
	}
	for i, cc := range cfg.ButtonCCs {
		if cc > 127 {
			return cfg, fmt.Errorf("button_ccs[%d] must be 0-127, got %d", i, cc)
		}
	}
	if cfg.EncoderCC > 127 {
		return cfg, fmt.Errorf("encoder_cc must be 0-127, got %d", cfg.EncoderCC)
	}
	if cfg.TouchStripCC > 127 {
		return cfg, fmt.Errorf("touch_strip_cc must be 0-127, got %d", cfg.TouchStripCC)
	}
	if cfg.TouchStripCC2 > 127 {
		return cfg, fmt.Errorf("touch_strip_cc_2 must be 0-127, got %d", cfg.TouchStripCC2)
	}
	if cfg.TouchStripRelease != "hold" && cfg.TouchStripRelease != "zero" && cfg.TouchStripRelease != "center" {
		return cfg, fmt.Errorf("touch_strip_release must be hold, zero, or center, got %q", cfg.TouchStripRelease)
	}
	if cfg.TouchStripMin >= cfg.TouchStripMax {
		return cfg, fmt.Errorf("touch_strip_min must be less than touch_strip_max")
	}
	if cfg.TouchStripDeadzone > cfg.TouchStripMax-cfg.TouchStripMin {
		return cfg, fmt.Errorf("touch_strip_deadzone must fit inside touch strip min/max range")
	}

	if _, err := parseColor(cfg.PadColorActive); err != nil {
		return cfg, fmt.Errorf("pad_color_active: %w", err)
	}
	if _, err := parseColor(cfg.PadColorIdle); err != nil {
		return cfg, fmt.Errorf("pad_color_idle: %w", err)
	}
	if _, err := parseColorLevel(cfg.PadLevelActive); err != nil {
		return cfg, fmt.Errorf("pad_level_active: %w", err)
	}
	if _, err := parseColorLevel(cfg.PadLevelIdle); err != nil {
		return cfg, fmt.Errorf("pad_level_idle: %w", err)
	}
	for i, color := range cfg.PadColors {
		if _, err := parseColor(color); err != nil {
			return cfg, fmt.Errorf("pad_colors[%d]: %w", i, err)
		}
	}
	for i, level := range cfg.PadLevels {
		if _, err := parseColorLevel(level); err != nil {
			return cfg, fmt.Errorf("pad_levels[%d]: %w", i, err)
		}
	}
	for i, intensity := range cfg.ButtonLEDs {
		if _, err := parseIntensity(intensity); err != nil {
			return cfg, fmt.Errorf("button_leds[%d]: %w", i, err)
		}
	}
	if _, err := parseIntensity(cfg.ButtonLEDIdle); err != nil {
		return cfg, fmt.Errorf("button_led_idle: %w", err)
	}
	if cfg.DisplayMode != "trigger" && cfg.DisplayMode != "name" && cfg.DisplayMode != "global" && cfg.DisplayMode != "off" {
		return cfg, fmt.Errorf("display_mode must be trigger, name, global, or off, got %q", cfg.DisplayMode)
	}

	return cfg, nil
}

func saveConfig(path string, cfg Config) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating config: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
