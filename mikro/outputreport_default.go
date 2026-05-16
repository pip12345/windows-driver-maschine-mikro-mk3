//go:build !windows

package mikro

import "github.com/karalabe/hid"

func writeOutputReport(device hid.Device, _ string, report []byte) (int, error) {
	return device.Write(report)
}
