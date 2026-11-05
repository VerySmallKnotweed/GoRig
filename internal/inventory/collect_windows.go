//go:build windows

package inventory

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"

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
	Name                      string
	Manufacturer              string
	NumberOfCores             uint32
	NumberOfLogicalProcessors uint32
	MaxClockSpeed             uint32
}

type diskDriveWMI struct {
	Name          string
	Manufacturer  string
	Model         string
	SerialNumber  string
	Size          uint64
	InterfaceType string
	PNPDeviceID   string
	MediaType     string
}

type biosWMI struct {
	SerialNumber string
}

func collectOS() report.Value {
	cmd := exec.Command("cmd.exe", "/d", "/c", "ver")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.Output()
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
	err := queryInventory(
		"SELECT Name, Manufacturer, NumberOfCores, NumberOfLogicalProcessors, MaxClockSpeed FROM Win32_Processor",
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
			Attributes: map[string]report.Value{
				"cores":              wmiNumber(uint64(processor.NumberOfCores), "WMI Win32_Processor.NumberOfCores"),
				"logical_processors": wmiNumber(uint64(processor.NumberOfLogicalProcessors), "WMI Win32_Processor.NumberOfLogicalProcessors"),
				"max_clock_mhz":      wmiNumber(uint64(processor.MaxClockSpeed), "WMI Win32_Processor.MaxClockSpeed"),
			},
		})
	}

	return components
}

func collectDiskDrives() []report.Component {
	var drives []diskDriveWMI
	err := queryInventory(
		"SELECT Name, Manufacturer, Model, SerialNumber, Size, InterfaceType, MediaType, PNPDeviceID FROM Win32_DiskDrive",
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
			Attributes: map[string]report.Value{
				"capacity_bytes": wmiNumber(drive.Size, "WMI Win32_DiskDrive.Size"),
				"interface_type": wmiString(drive.InterfaceType, "WMI Win32_DiskDrive.InterfaceType"),
				"pnp_device_id":  wmiString(drive.PNPDeviceID, "WMI Win32_DiskDrive.PNPDeviceID"),
				"media_type":     wmiString(drive.MediaType, "WMI Win32_DiskDrive.MediaType"),
			},
		})
	}

	return components
}

// Null properties are common in firmware; leave them as zero values rather
// than failing the entire query when an individual field is missing.
var nullableWMI = &wmi.Client{NonePtrZero: true}

var queryInventory = func(query string, dst interface{}) error {
	return nullableWMI.Query(query, dst)
}

type physicalMemoryWMI struct {
	Manufacturer  string
	PartNumber    string
	SerialNumber  string
	DeviceLocator string
	Capacity      uint64
	Speed         uint32
}

type baseboardWMI struct {
	Manufacturer string
	Product      string
	SerialNumber string
}

type videoControllerWMI struct {
	Name                 string
	AdapterCompatibility string
	DriverVersion        string
	PNPDeviceID          string
	AdapterRAM           uint32
}

// queryFailure represents an unsuccessful query without confusing it with
// an actually installed component. Details remain in the observations.
func queryFailure(componentType, source string, err error) []report.Component {
	observation := report.Observation{Source: source, Status: "unavailable", Raw: "query returned no instances"}
	if err != nil {
		observation.Status = "error"
		observation.Raw = err.Error()
	}
	value := selectPrimaryValue([]report.Observation{observation})
	return []report.Component{{Type: componentType, Name: value,
		Manufacturer: value, Model: value, SerialNumber: value}}
}

func wmiString(value, source string) report.Value {
	return selectPrimaryValue([]report.Observation{makeObservation(value, source)})
}

// Zero is also how WMI null/unspecified numeric properties are decoded.
func wmiNumber(value uint64, source string) report.Value {
	if value == 0 {
		return wmiString("", source)
	}
	return wmiString(fmt.Sprintf("%d", value), source)
}

func collectMemory() []report.Component {
	var modules []physicalMemoryWMI
	err := queryInventory("SELECT Manufacturer, PartNumber, SerialNumber, DeviceLocator, Capacity, Speed FROM Win32_PhysicalMemory", &modules)
	if err != nil || len(modules) == 0 {
		return queryFailure("memory", "WMI Win32_PhysicalMemory", err)
	}
	components := make([]report.Component, 0, len(modules))
	for _, module := range modules {
		components = append(components, report.Component{
			Type:         "memory",
			Name:         wmiString(module.DeviceLocator, "WMI Win32_PhysicalMemory.DeviceLocator"),
			Manufacturer: wmiString(module.Manufacturer, "WMI Win32_PhysicalMemory.Manufacturer"),
			Model:        wmiString(module.PartNumber, "WMI Win32_PhysicalMemory.PartNumber"),
			SerialNumber: wmiString(module.SerialNumber, "WMI Win32_PhysicalMemory.SerialNumber"),
			Attributes: map[string]report.Value{
				"capacity_bytes": wmiNumber(module.Capacity, "WMI Win32_PhysicalMemory.Capacity"),
				"speed_mhz":      wmiNumber(uint64(module.Speed), "WMI Win32_PhysicalMemory.Speed"),
			},
		})
	}
	return components
}

func collectBaseboards() []report.Component {
	var boards []baseboardWMI
	err := queryInventory("SELECT Manufacturer, Product, SerialNumber FROM Win32_BaseBoard", &boards)
	if err != nil || len(boards) == 0 {
		return queryFailure("motherboard", "WMI Win32_BaseBoard", err)
	}
	components := make([]report.Component, 0, len(boards))
	for _, board := range boards {
		components = append(components, report.Component{
			Type:         "motherboard",
			Name:         wmiString(board.Product, "WMI Win32_BaseBoard.Product"),
			Manufacturer: wmiString(board.Manufacturer, "WMI Win32_BaseBoard.Manufacturer"),
			Model:        wmiString(board.Product, "WMI Win32_BaseBoard.Product"),
			SerialNumber: wmiString(board.SerialNumber, "WMI Win32_BaseBoard.SerialNumber"),
		})
	}
	return components
}

func collectVideoControllers() []report.Component {
	var controllers []videoControllerWMI
	err := queryInventory("SELECT Name, AdapterCompatibility, DriverVersion, PNPDeviceID, AdapterRAM FROM Win32_VideoController", &controllers)
	if err != nil || len(controllers) == 0 {
		return queryFailure("video", "WMI Win32_VideoController", err)
	}
	components := make([]report.Component, 0, len(controllers))
	for _, controller := range controllers {
		components = append(components, report.Component{
			Type:         "video",
			Name:         wmiString(controller.Name, "WMI Win32_VideoController.Name"),
			Manufacturer: wmiString(controller.AdapterCompatibility, "WMI Win32_VideoController.AdapterCompatibility"),
			Model:        report.Value{Status: "not_collected"},
			SerialNumber: report.Value{Status: "not_collected"},
			Attributes: map[string]report.Value{
				"driver_version":     wmiString(controller.DriverVersion, "WMI Win32_VideoController.DriverVersion"),
				"pnp_device_id":      wmiString(controller.PNPDeviceID, "WMI Win32_VideoController.PNPDeviceID"),
				"video_memory_bytes": wmiNumber(uint64(controller.AdapterRAM), "WMI Win32_VideoController.AdapterRAM"),
			},
		})
	}
	return components
}
