package mikro

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=Intensity -trimprefix=Intensity
type Intensity uint8

const (
	IntensityLow Intensity = iota
	IntensityOff
	IntensityMedium
	IntensityHigh
)

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=Color -trimprefix=Color
type Color uint8

const (
	ColorOff Color = iota
	ColorRed
	ColorOrange
	ColorLightOrange
	ColorWarmYellow
	ColorYellow
	ColorLime
	ColorGreen
	ColorMint
	ColorCyan
	ColorTurquoise
	ColorBlue
	ColorPlum
	ColorViolet
	ColorPurple
	ColorMagenta
	ColorFuchsia
	ColorWhite
)

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=ColorLevel -trimprefix=ColorLevel
type ColorLevel uint8

const (
	ColorLevelLow ColorLevel = iota
	ColorLevelMedium
	ColorLevelHigh
	ColorLevelFaded
)

type ColoredLight struct {
	Level ColorLevel
	Color Color
}

type Lights struct {
	Buttons [39]Intensity
	Pads    [16]ColoredLight
	Strip   [25]ColoredLight
}

func NewLights() Lights {
	l := Lights{}

	for idx := range l.Buttons {
		l.Buttons[idx] = IntensityOff
	}

	return l
}

func (l Lights) Encode() []byte {
	encoded := make([]byte, 81)
	encoded[0] = 0x80

	for idx, intensity := range l.Buttons {
		encoded[1+idx] = buttonIntensityByte(intensity)
	}

	for idx, light := range l.Pads {
		encoded[40+hardwarePadIndex(idx)] = padLightByte(light)
	}

	for idx, light := range l.Strip {
		encoded[56+idx] = padLightByte(light)
	}

	return encoded
}

func hardwarePadIndex(idx int) int {
	row := idx / 4
	col := idx % 4
	return (3-row)*4 + col
}

func buttonIntensityByte(intensity Intensity) byte {
	switch intensity {
	case IntensityLow:
		return 0x7c
	case IntensityMedium:
		return 0x7e
	case IntensityHigh:
		return 0x7f
	default:
		return 0
	}
}

func padLightByte(light ColoredLight) byte {
	if light.Color == ColorOff {
		return 0
	}

	return byte(light.Color<<2) | colorLevelByte(light.Level)
}

func colorLevelByte(level ColorLevel) byte {
	switch level {
	case ColorLevelLow, ColorLevelFaded:
		return 0
	case ColorLevelMedium:
		return 2
	case ColorLevelHigh:
		return 3
	default:
		return 0
	}
}
