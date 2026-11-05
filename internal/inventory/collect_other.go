//go:build !windows

package inventory

import (
	"runtime"

	"GoRig/internal/report"
)

func collectOS() report.Value {
	osName := runtime.GOOS
	return report.Value{
		Value:  osName,
		Raw:    runtime.GOOS,
		Source: "runtime.GOOS",
		Status: "collected",
	}
}

// The hardware collector currently supports Windows only.
func collectHardwareIdentity() (report.Value, report.Value, report.Value) {
	v := report.Value{Status: "not_collected"}
	return v, v, v
}
func collectProcessors() []report.Component       { return nil }
func collectDiskDrives() []report.Component       { return nil }
func collectMemory() []report.Component           { return nil }
func collectBaseboards() []report.Component       { return nil }
func collectVideoControllers() []report.Component { return nil }

func collectDeviceAttributes() map[string]report.Value { return nil }
func collectNetworkAdapters() []report.Component       { return nil }
func collectFirmware() []report.Component              { return nil }

func collectInstalledMemory(_ []report.Component) report.Value {
	return report.Value{Status: "not_collected"}
}
func collectLogicalVolumes() []report.Component { return nil }
func collectPeripherals() []report.Component    { return nil }

func collectUSBDevices() []report.Component { return nil }
