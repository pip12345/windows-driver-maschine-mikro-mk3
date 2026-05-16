package main

import "essaim.dev/mikro"

// MIDI status bytes
const (
	midiNoteOff = 0x80
	midiNoteOn  = 0x90
)

// Default pad-to-MIDI-note mapping (GM drum map, starting at kick drum).
// Layout matches physical pad grid (bottom-left = Pad1 = index 0):
//
//	Row 4: Pad13 Pad14 Pad15 Pad16
//	Row 3: Pad9  Pad10 Pad11 Pad12
//	Row 2: Pad5  Pad6  Pad7  Pad8
//	Row 1: Pad1  Pad2  Pad3  Pad4
var defaultPadNotes = [16]uint8{
	36, 37, 38, 39, // Row 1: Kick, Side Stick, Snare, Hand Clap
	40, 41, 42, 43, // Row 2: Snare 2, Low Tom, Closed HH, Low Tom 2
	44, 45, 46, 47, // Row 3: Pedal HH, Mid Tom, Open HH, Mid Tom 2
	48, 49, 50, 51, // Row 4: Hi Tom, Crash, Ride, Crash 2
}

// padIndex maps mikro.Pad constants to 0-15 indices.
// The mikro library uses named constants (PadNumber1..PadNumber16) but the underlying
// values may not be 0-15, so we map them explicitly.
var padIndex = map[mikro.Pad]int{
	mikro.PadNumber1:  0,
	mikro.PadNumber2:  1,
	mikro.PadNumber3:  2,
	mikro.PadNumber4:  3,
	mikro.PadNumber5:  4,
	mikro.PadNumber6:  5,
	mikro.PadNumber7:  6,
	mikro.PadNumber8:  7,
	mikro.PadNumber9:  8,
	mikro.PadNumber10: 9,
	mikro.PadNumber11: 10,
	mikro.PadNumber12: 11,
	mikro.PadNumber13: 12,
	mikro.PadNumber14: 13,
	mikro.PadNumber15: 14,
	mikro.PadNumber16: 15,
}

// scaleVelocity converts mikro's 12-bit velocity (0-4095) to MIDI velocity (0-127).
func scaleVelocity(v uint16) uint8 {
	if v == 0 {
		return 0
	}
	vel := (uint32(v)*127 + 2047) / 4095
	if vel > 127 {
		vel = 127
	}
	if vel == 0 {
		vel = 1 // ensure non-zero press produces audible note
	}
	return uint8(vel)
}

// noteOn returns a 3-byte MIDI Note On message.
func noteOn(channel, note, velocity uint8) []byte {
	return []byte{midiNoteOn | (channel & 0x0F), note & 0x7F, velocity & 0x7F}
}

// noteOff returns a 3-byte MIDI Note Off message.
func noteOff(channel, note uint8) []byte {
	return []byte{midiNoteOff | (channel & 0x0F), note & 0x7F, 0}
}
