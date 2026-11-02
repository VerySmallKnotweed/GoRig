//go:build windows

package inventory

import (
	"fmt"
	"os/exec"
	"strings"

	"GoRig/internal/report"

	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows/registry"
)

const biosRegistryPath = `HARDWARE\DESCRIPTION\System\BIOS`

type computerSystemWMI struct {
	Manufacturer string
	Model        string
}

type processorWMI struct {
	Name         string
	Manufacturer string
}

type diskDriveWMI struct {
	Name         string
	Manufacturer string
	Model        string
	SerialNumber string
}

type biosWMI struct {
	SerialNumber string
}

func collectOS() report.Value {
	output, err := exec.Command("cmd.exe", "/d", "/c", "ver").Output()
	if err != nil {
		return report.Value{
			Value:  "Windows",
			Source: "cmd.exe /d /c ver",
			Status: "unavailable",
		}
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return report.Value{
			Value:  "Windows",
			Source: "cmd.exe /d /c ver",
			Status: "unavailable",
		}
	}

	return report.Value{
		Value:  "Windows",
		Raw:    raw,
		Source: "cmd.exe /d /c ver",
		Status: "collected",
	}
}

func collectHardwareIdentity() (manufacturer, model, serialNumber report.Value) {
	manufacturerObs, modelObs, serialObs := collectWMIdentity()

	manufacturerObs = append(
		manufacturerObs,
		readBIOSRegistryValue("SystemManufacturer"),
	)
	modelObs = append(
		modelObs,
		readBIOSRegistryValue("SystemProductName"),
	)
	serialObs = append(
		serialObs,
		readBIOSRegistryValue("SystemSerialNumber"),
	)

	return selectPrimaryValue(manufacturerObs),
		selectPrimaryValue(modelObs),
		selectPrimaryValue(serialObs)
}

func collectWMIdentity() (
	manufacturerObs []report.Observation,
	modelObs []report.Observation,
	serialObs []report.Observation,
) {
	var systems []computerSystemWMI
	err := wmi.Query(
		"SELECT Manufacturer, Model FROM Win32_ComputerSystem",
		&systems,
	)
	if err != nil {
		observation := report.Observation{
			Source: "WMI Win32_ComputerSystem",
			Status: "error",
			Raw:    err.Error(),
		}
		manufacturerObs = append(manufacturerObs, observation)
		modelObs = append(modelObs, observation)
	} else if len(systems) == 0 {
		observation := report.Observation{
			Source: "WMI Win32_ComputerSystem",
			Status: "unavailable",
			Raw:    "query returned no instances",
		}
		manufacturerObs = append(manufacturerObs, observation)
		modelObs = append(modelObs, observation)
	} else {
		manufacturerObs = append(manufacturerObs, makeObservation(
			systems[0].Manufacturer,
			"WMI Win32_ComputerSystem.Manufacturer",
		))
		modelObs = append(modelObs, makeObservation(
			systems[0].Model,
			"WMI Win32_ComputerSystem.Model",
		))
	}

	var bios []biosWMI
	err = wmi.Query(
		"SELECT SerialNumber FROM Win32_BIOS",
		&bios,
	)
	if err != nil {
		serialObs = append(serialObs, report.Observation{
			Source: "WMI Win32_BIOS.SerialNumber",
			Status: "error",
			Raw:    err.Error(),
		})
	} else if len(bios) == 0 {
		serialObs = append(serialObs, report.Observation{
			Source: "WMI Win32_BIOS.SerialNumber",
			Status: "unavailable",
			Raw:    "query returned no instances",
		})
	} else {
		serialObs = append(serialObs, makeObservation(
			bios[0].SerialNumber,
			"WMI Win32_BIOS.SerialNumber",
		))
	}

	return manufacturerObs, modelObs, serialObs
}

func readBIOSRegistryValue(name string) report.Observation {
	source := fmt.Sprintf(
		`HKLM\%s\%s`,
		biosRegistryPath,
		name,
	)

	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		biosRegistryPath,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return report.Observation{
			Source: source,
			Status: "error",
			Raw:    err.Error(),
		}
	}
	defer key.Close()

	value, _, err := key.GetStringValue(name)
	if err != nil {
		return report.Observation{
			Source: source,
			Status: "unavailable",
			Raw:    err.Error(),
		}
	}

	return makeObservation(value, source)
}

func makeObservation(value, source string) report.Observation {
	if strings.TrimSpace(value) == "" {
		return report.Observation{
			Source: source,
			Status: "unavailable",
			Raw:    value,
		}
	}

	status := "collected"
	if strings.EqualFold(strings.TrimSpace(value), "Default string") {
		status = "template"
	}

	return report.Observation{
		Value:  value,
		Raw:    value,
		Source: source,
		Status: status,
	}
}

func selectPrimaryValue(observations []report.Observation) report.Value {
	result := report.Value{
		Status:       "unavailable",
		Observations: observations,
	}

	// Сначала выбирается непустое значение, не помеченное как шаблонное.
	for _, observation := range observations {
		if observation.Status == "collected" {
			result.Value = observation.Value
			result.Raw = observation.Raw
			result.Source = observation.Source
			result.Status = "collected"
			return result
		}
	}

	// Если доступны только шаблонные значения, сохраняем первое как основное,
	// но явно помечаем его статусом template.
	for _, observation := range observations {
		if observation.Status == "template" {
			result.Value = observation.Value
			result.Raw = observation.Raw
			result.Source = observation.Source
			result.Status = "template"
			return result
		}
	}

	if len(observations) > 0 {
		result.Source = observations[0].Source
	}
	return result
}

func collectProcessors() []report.Component {
	var processors []processorWMI
	err := wmi.Query(
		"SELECT Name, Manufacturer FROM Win32_Processor",
		&processors,
	)
	if err != nil {
		errorObservation := report.Observation{
			Source: "WMI Win32_Processor",
			Status: "error",
			Raw:    err.Error(),
		}

		return []report.Component{{
			Type:         "processor",
			Name:         selectPrimaryValue([]report.Observation{errorObservation}),
			Manufacturer: selectPrimaryValue([]report.Observation{errorObservation}),
			Model:        report.Value{Status: "not_collected"},
			SerialNumber: report.Value{Status: "not_collected"},
		}}
	}

	if len(processors) == 0 {
		observation := report.Observation{
			Source: "WMI Win32_Processor",
			Status: "unavailable",
			Raw:    "query returned no instances",
		}

		return []report.Component{{
			Type:         "processor",
			Name:         selectPrimaryValue([]report.Observation{observation}),
			Manufacturer: selectPrimaryValue([]report.Observation{observation}),
			Model:        report.Value{Status: "not_collected"},
			SerialNumber: report.Value{Status: "not_collected"},
		}}
	}

	components := make([]report.Component, 0, len(processors))
	for _, processor := range processors {
		nameSource := "WMI Win32_Processor.Name"
		manufacturerSource := "WMI Win32_Processor.Manufacturer"

		components = append(components, report.Component{
			Type:         "processor",
			Name:         selectPrimaryValue([]report.Observation{makeObservation(processor.Name, nameSource)}),
			Manufacturer: selectPrimaryValue([]report.Observation{makeObservation(processor.Manufacturer, manufacturerSource)}),
			Model:        report.Value{Status: "not_collected"},
			SerialNumber: report.Value{Status: "not_collected"},
		})
	}

	return components
}

func collectDiskDrives() []report.Component {
	var drives []diskDriveWMI
	err := wmi.Query(
		"SELECT Name, Manufacturer, Model, SerialNumber FROM Win32_DiskDrive",
		&drives,
	)

	if err != nil {
		observation := report.Observation{
			Source: "WMI Win32_DiskDrive",
			Status: "error",
			Raw:    err.Error(),
		}

		return []report.Component{{
			Type:         "disk",
			Name:         selectPrimaryValue([]report.Observation{observation}),
			Manufacturer: selectPrimaryValue([]report.Observation{observation}),
			Model:        selectPrimaryValue([]report.Observation{observation}),
			SerialNumber: selectPrimaryValue([]report.Observation{observation}),
		}}
	}

	if len(drives) == 0 {
		observation := report.Observation{
			Source: "WMI Win32_DiskDrive",
			Status: "unavailable",
			Raw:    "query returned no instances",
		}

		return []report.Component{{
			Type:         "disk",
			Name:         selectPrimaryValue([]report.Observation{observation}),
			Manufacturer: selectPrimaryValue([]report.Observation{observation}),
			Model:        selectPrimaryValue([]report.Observation{observation}),
			SerialNumber: selectPrimaryValue([]report.Observation{observation}),
		}}
	}

	components := make([]report.Component, 0, len(drives))
	for _, drive := range drives {
		components = append(components, report.Component{
			Type: "disk",
			Name: selectPrimaryValue([]report.Observation{
				makeObservation(drive.Name, "WMI Win32_DiskDrive.Name"),
			}),
			Manufacturer: selectPrimaryValue([]report.Observation{
				makeObservation(drive.Manufacturer, "WMI Win32_DiskDrive.Manufacturer"),
			}),
			Model: selectPrimaryValue([]report.Observation{
				makeObservation(drive.Model, "WMI Win32_DiskDrive.Model"),
			}),
			SerialNumber: selectPrimaryValue([]report.Observation{
				makeObservation(drive.SerialNumber, "WMI Win32_DiskDrive.SerialNumber"),
			}),
		})
	}

	return components
}
