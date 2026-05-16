//go:build !windows

package main

import "fmt"

func initVirtualMIDI() error {
	return fmt.Errorf("virtualMIDI is only supported on Windows")
}

type VirtualMIDIPort struct{}

func CreateVirtualMIDIPort(name string) (*VirtualMIDIPort, error) {
	return nil, fmt.Errorf("virtualMIDI is only supported on Windows")
}

func (p *VirtualMIDIPort) SendData(data []byte) error {
	return fmt.Errorf("virtualMIDI is only supported on Windows")
}

func (p *VirtualMIDIPort) Close() {}
