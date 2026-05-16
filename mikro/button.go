package mikro

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=Button -trimprefix Button
type Button int

const (
	// Browser Section

	ButtonProject Button = iota
	ButtonFavorites
	ButtonBrowser

	// Edit section

	ButtonVolume
	ButtonSwing
	ButtonTempo
	ButtonPlugin
	ButtonSampling

	ButtonArrowLeft
	ButtonArrowRight

	// Performance section

	ButtonPitch
	ButtonMod
	ButtonPerform
	ButtonNotes

	ButtonGroup
	ButtonAuto
	ButtonLock
	ButtonNoteRepeat

	// Transport section

	ButtonRestart
	ButtonErase
	ButtonTap
	ButtonFollow

	ButtonPlay
	ButtonRec
	ButtonStop
	ButtonShift

	// Pads section

	ButtonFixedVel
	ButtonPadMode
	ButtonKeyboard
	ButtonChords
	ButtonStep

	ButtonScene
	ButtonPattern
	ButtonEvents
	ButtonVariation
	ButtonDuplicate
	ButtonSelect
	ButtonSolo
	ButtonMute

	ButtonEncoder
)

type ButtonMessage struct {
	pressed uint64 // bitflag of all currently pressed buttons

	encoderTouched  bool
	encoderPosition uint8

	stripPos1 uint8
	stripPos2 uint8
}

func (b *ButtonMessage) IsButtonPressed(btn Button) bool {
	return b.pressed&(1<<btn) != 0
}

func (b *ButtonMessage) PressedButtons() []Button {
	var pressed []Button

	for idx := 0; idx < 64; idx++ {
		if b.pressed&(1<<idx) != 0 {
			pressed = append(pressed, Button(idx))
		}
	}

	return pressed
}

func (b *ButtonMessage) IsEncoderTouched() bool {
	return b.encoderTouched
}

func (b *ButtonMessage) EncoderPosition() uint8 {
	return b.encoderPosition
}

func (b *ButtonMessage) StripPosition() uint8 {
	return b.stripPos1
}

func (b *ButtonMessage) StripSecondPosition() uint8 {
	return b.stripPos2
}
