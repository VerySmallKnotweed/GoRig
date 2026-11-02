package inventory

import (
	"os"

	"GoRig/internal/report"
)

func Collect(collectorVersion string) (report.Report, error) {
	result, err := report.New(collectorVersion)
	if err != nil {
		return report.Report{}, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		result.Device.ComputerName = report.Value{
			Status: "unavailable",
			Source: "os.Hostname",
		}
	} else {
		result.Device.ComputerName = report.Value{
			Value:  hostname,
			Raw:    hostname,
			Source: "os.Hostname",
			Status: "collected",
		}
	}

	result.Device.OS = collectOS()
	result.Device.Attributes = collectDeviceAttributes()

	result.Device.Manufacturer,
		result.Device.Model,
		result.Device.SerialNumber = collectHardwareIdentity()

	result.Device.Components = append(
		collectProcessors(),
		collectDiskDrives()...,
	)

	result.Device.Components = append(result.Device.Components, collectMemory()...)
	result.Device.Components = append(result.Device.Components, collectBaseboards()...)
	result.Device.Components = append(result.Device.Components, collectVideoControllers()...)
	result.Device.Components = append(result.Device.Components, collectNetworkAdapters()...)
	result.Device.Components = append(result.Device.Components, collectFirmware()...)

	return result, nil
}
