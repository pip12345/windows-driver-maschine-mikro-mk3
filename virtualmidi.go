//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// virtualMIDI SDK DLL wrapper.
// Requires the virtualMIDI driver to be installed (comes with loopMIDI or standalone SDK).
// See: https://www.tobias-erichsen.de/software/virtualmidi/virtualmidi-sdk.html

const (
	// teVirtualMIDI flags
	vmFlagParseRX         = 1
	vmFlagParseTX         = 2
	vmFlagInstantiateRX   = 4
	vmFlagInstantiateTX   = 8
	vmFlagInstantiateBoth = vmFlagInstantiateRX | vmFlagInstantiateTX
)

var (
	dllVirtualMIDI *syscall.LazyDLL

	procCreatePortEx3 *syscall.LazyProc
	procClosePort     *syscall.LazyProc
	procSendData      *syscall.LazyProc
)

func initVirtualMIDI() error {
	dllVirtualMIDI = syscall.NewLazyDLL("teVirtualMIDI64.dll")
	if err := dllVirtualMIDI.Load(); err != nil {
		return fmt.Errorf("failed to load teVirtualMIDI64.dll: %w\n\nMake sure the virtualMIDI driver is installed.\nGet it from: https://www.tobias-erichsen.de/software/loopmidi.html", err)
	}

	procCreatePortEx3 = dllVirtualMIDI.NewProc("virtualMIDICreatePortEx3")
	procClosePort = dllVirtualMIDI.NewProc("virtualMIDIClosePort")
	procSendData = dllVirtualMIDI.NewProc("virtualMIDISendData")
	if err := procCreatePortEx3.Find(); err != nil {
		return fmt.Errorf("virtualMIDI DLL does not export virtualMIDICreatePortEx3: %w", err)
	}
	if err := procClosePort.Find(); err != nil {
		return fmt.Errorf("virtualMIDI DLL does not export virtualMIDIClosePort: %w", err)
	}
	if err := procSendData.Find(); err != nil {
		return fmt.Errorf("virtualMIDI DLL does not export virtualMIDISendData: %w", err)
	}

	return nil
}

// VirtualMIDIPort represents an open virtual MIDI port.
type VirtualMIDIPort struct {
	handle uintptr
	name   string
}

// CreateVirtualMIDIPort creates a new virtual MIDI port visible to DAWs and other MIDI software.
func CreateVirtualMIDIPort(name string) (*VirtualMIDIPort, error) {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("invalid port name: %w", err)
	}

	// virtualMIDICreatePortEx3(portName, callback, driverInstance, maxSysexLength, flags, manufacturer, product)
	// We pass NULL for callback and driverInstance (we only send, not receive).
	handle, _, callErr := procCreatePortEx3.Call(
		uintptr(unsafe.Pointer(namePtr)), // port name (LPCWSTR)
		0,                                // callback (NULL - we don't receive MIDI)
		0,                                // driverInstance (NULL)
		65535,                            // maxSysexLength
		uintptr(vmFlagInstantiateTX|vmFlagParseTX), // this app only sends MIDI
		0, // manufacturer GUID (NULL - use virtualMIDI default)
		0, // product GUID (NULL - use virtualMIDI default)
	)
	if handle == 0 {
		return nil, fmt.Errorf("virtualMIDICreatePortEx3 failed: %v", callErr)
	}

	return &VirtualMIDIPort{handle: handle, name: name}, nil
}

// SendData sends raw MIDI bytes through the virtual port.
func (p *VirtualMIDIPort) SendData(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	ret, _, callErr := procSendData.Call(
		p.handle,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
	)
	if ret == 0 {
		return fmt.Errorf("virtualMIDISendData failed: %v", callErr)
	}
	return nil
}

// Close closes the virtual MIDI port.
func (p *VirtualMIDIPort) Close() {
	if p.handle != 0 {
		procClosePort.Call(p.handle)
		p.handle = 0
	}
}
