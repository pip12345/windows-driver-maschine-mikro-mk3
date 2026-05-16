//go:build windows

package mikro

import (
	"fmt"
	"runtime"
	"syscall"

	"github.com/karalabe/hid"
)

const (
	genericWrite = 0x40000000
	shareRead    = 0x00000001
	shareWrite   = 0x00000002
	openExisting = 3
)

func writeOutputReport(_ hid.Device, path string, report []byte) (int, error) {
	paddedReport := make([]byte, 265)
	copy(paddedReport, report)
	report = paddedReport

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("invalid HID path: %w", err)
	}

	handle, err := syscall.CreateFile(
		pathPtr,
		genericWrite,
		shareRead|shareWrite,
		nil,
		openExisting,
		0,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("open HID output report: %w", err)
	}
	defer syscall.CloseHandle(handle)

	var written uint32
	if err := syscall.WriteFile(handle, report, &written, nil); err != nil {
		return int(written), fmt.Errorf("write HID output report: %w", err)
	}
	if int(written) != len(report) {
		return int(written), fmt.Errorf("short HID output report write: %d/%d", written, len(report))
	}

	runtime.KeepAlive(report)
	return int(written), nil
}
