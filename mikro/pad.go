package mikro

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=Pad -trimprefix=Pad
type Pad uint8

const (
	// Pads by indicated number

	PadNumber13 Pad = iota
	PadNumber14
	PadNumber15
	PadNumber16

	PadNumber9
	PadNumber10
	PadNumber11
	PadNumber12

	PadNumber5
	PadNumber6
	PadNumber7
	PadNumber8

	PadNumber1
	PadNumber2
	PadNumber3
	PadNumber4

	// Pads by name

	PadUndo     = PadNumber1
	PadRedo     = PadNumber2
	PadStepUndo = PadNumber3
	PadStepRedo = PadNumber4

	PadQuantize   = PadNumber5
	PadQuantize50 = PadNumber6
	PadNudgeLeft  = PadNumber7
	PadNudgeRight = PadNumber8

	PadClear     = PadNumber9
	PadClearAuto = PadNumber10
	PadCopy      = PadNumber11
	PadPaste     = PadNumber12

	PadSemitoneMinus = PadNumber13
	PadSemitonePlus  = PadNumber14
	PadOctaveMinus   = PadNumber15
	PadOctavePlus    = PadNumber16
)

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=PadAction -trimprefix=PadAction
type PadAction uint8

const (
	PadActionPressed PadAction = iota + 1
	PadActionTouched
	PadActionReleased
)

type PadMessage struct {
	pad      Pad
	velocity uint16
	action   PadAction
}

func (p *PadMessage) Pad() Pad {
	return p.pad
}

func (p *PadMessage) Velocity() uint16 {
	return p.velocity
}

func (p *PadMessage) Action() PadAction {
	return p.action
}
