//go:build windows

package inventory

import (
	"fmt"
	"strings"
	"testing"

	"GoRig/internal/report"
)

func TestSelectPrimaryValue(t *testing.T) {
	tests := []struct {
		name         string
		observations []report.Observation
		wantValue    string
		wantStatus   string
		wantSource   string
	}{
		{
			name: "обычное значение важнее шаблонного",
			observations: []report.Observation{
				{
					Value:  "Default string",
					Source: "WMI",
					Status: "template",
				},
				{
					Value:  "Example Maker",
					Source: "Registry",
					Status: "collected",
				},
			},
			wantValue:  "Example Maker",
			wantStatus: "collected",
			wantSource: "Registry",
		},
		{
			name: "сохраняет шаблон, если других значений нет",
			observations: []report.Observation{
				{
					Value:  "Default string",
					Source: "WMI",
					Status: "template",
				},
			},
			wantValue:  "Default string",
			wantStatus: "template",
			wantSource: "WMI",
		},
		{
			name: "нет доступных значений",
			observations: []report.Observation{
				{
					Source: "Registry",
					Status: "unavailable",
				},
			},
			wantValue:  "",
			wantStatus: "unavailable",
			wantSource: "Registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectPrimaryValue(tt.observations)

			if got.Value != tt.wantValue ||
				got.Status != tt.wantStatus ||
				got.Source != tt.wantSource {
				t.Errorf(
					"получено Value=%q Status=%q Source=%q; ожидалось Value=%q Status=%q Source=%q",
					got.Value, got.Status, got.Source,
					tt.wantValue, tt.wantStatus, tt.wantSource,
				)
			}

			if len(got.Observations) != len(tt.observations) {
				t.Errorf(
					"сохранено наблюдений: %d; ожидалось: %d",
					len(got.Observations), len(tt.observations),
				)
			}
		})
	}
}

func TestMakeObservationMarksTemplate(t *testing.T) {
	got := makeObservation("Default string", "WMI")

	if got.Status != "template" {
		t.Errorf("статус = %q; ожидался template", got.Status)
	}
	if got.Value != "Default string" || got.Raw != "Default string" {
		t.Errorf("исходное значение изменилось: %+v", got)
	}
}

func TestWMICategoryFailure(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status string
	}{
		{nil, "unavailable"},
		{fmt.Errorf("WMI failed"), "error"},
	} {
		components := queryFailure("memory", "WMI Win32_PhysicalMemory", tc.err)
		if len(components) != 1 || components[0].Type != "memory" {
			t.Fatalf("components: %+v", components)
		}
		observations := components[0].Name.Observations
		if len(observations) != 1 || observations[0].Status != tc.status {
			t.Fatalf("observations: %+v", observations)
		}
	}
}

func TestWMINumber(t *testing.T) {
	zero := wmiNumber(0, "WMI Capacity")
	if zero.Status != "unavailable" {
		t.Fatalf("zero: %+v", zero)
	}
	capacity := wmiNumber(17179869184, "WMI Capacity")
	if capacity.Value != "17179869184" || capacity.Status != "collected" {
		t.Fatalf("capacity: %+v", capacity)
	}
}

func TestNewComponentCollection(t *testing.T) {
	previous := queryInventory
	t.Cleanup(func() { queryInventory = previous })
	queryInventory = func(query string, dst interface{}) error {
		switch result := dst.(type) {
		case *[]physicalMemoryWMI:
			*result = []physicalMemoryWMI{
				{DeviceLocator: "DIMM 1", Manufacturer: "Maker", PartNumber: "Part", SerialNumber: "S1", Capacity: 8589934592, Speed: 3200},
				{DeviceLocator: "DIMM 2", Capacity: 4294967296},
			}
		case *[]baseboardWMI:
			*result = []baseboardWMI{{Manufacturer: "Maker", Product: "Board", SerialNumber: "B1"}}
		case *[]videoControllerWMI:
			*result = []videoControllerWMI{{Name: "GPU", AdapterCompatibility: "Vendor", PNPDeviceID: "PCI-ID", DriverVersion: "1.2"}}
		default:
			return fmt.Errorf("unexpected query: %s", query)
		}
		return nil
	}
	memory := collectMemory()
	if len(memory) != 2 || memory[0].Name.Value != "DIMM 1" || memory[0].Model.Value != "Part" || memory[0].Attributes["capacity_bytes"].Value != "8589934592" || memory[1].Attributes["speed_mhz"].Status != "unavailable" {
		t.Fatalf("memory: %+v", memory)
	}
	boards := collectBaseboards()
	if len(boards) != 1 || boards[0].Model.Value != "Board" || boards[0].SerialNumber.Value != "B1" {
		t.Fatalf("boards: %+v", boards)
	}
	video := collectVideoControllers()
	if len(video) != 1 || video[0].Name.Value != "GPU" || video[0].Attributes["pnp_device_id"].Value != "PCI-ID" || video[0].SerialNumber.Status != "not_collected" {
		t.Fatalf("video: %+v", video)
	}
}

func TestCollectionContinuesAfterCategoryError(t *testing.T) {
	previous := queryInventory
	t.Cleanup(func() { queryInventory = previous })
	queryInventory = func(query string, dst interface{}) error {
		switch result := dst.(type) {
		case *[]physicalMemoryWMI:
			return fmt.Errorf("WMI failed")
		case *[]baseboardWMI:
			*result = []baseboardWMI{{Product: "Board"}}
		}
		return nil
	}
	if got := collectMemory()[0].Name.Observations[0].Status; got != "error" {
		t.Fatalf("status: %s", got)
	}
	if got := collectBaseboards()[0].Model.Value; got != "Board" {
		t.Fatalf("board: %s", got)
	}
	if got := collectVideoControllers()[0].Name.Observations[0].Status; got != "unavailable" {
		t.Fatalf("empty status: %s", got)
	}
}

func TestExtendedInventory(t *testing.T) {
	oldDate, oldRegistry := readInstallDate, readRegistryString
	t.Cleanup(func() { readInstallDate, readRegistryString = oldDate, oldRegistry })
	readInstallDate = func() report.Observation { return report.Observation{Source: "registry", Status: "unavailable"} }
	readRegistryString = func(path, name string) report.Observation {
		if name == "DisplayVersion" {
			return report.Observation{Source: "registry", Status: "collected", Value: "23H2"}
		}
		return report.Observation{Source: "registry", Status: "unavailable"}
	}
	previous := queryInventory
	t.Cleanup(func() { queryInventory = previous })
	queryInventory = func(query string, dst interface{}) error {
		switch result := dst.(type) {
		case *[]computerMemoryWMI:
			*result = []computerMemoryWMI{{TotalPhysicalMemory: 17179869184}}
		case *[]osDetailsWMI:
			*result = []osDetailsWMI{{Caption: "Windows 11 Pro", Version: "10.0", BuildNumber: "22631", OSArchitecture: "64-bit", InstallDate: "20240625120100.000000+180"}}
		case *[]processorWMI:
			*result = []processorWMI{{Name: "CPU", NumberOfCores: 4, NumberOfLogicalProcessors: 8, MaxClockSpeed: 3200}}
		case *[]diskDriveWMI:
			*result = []diskDriveWMI{{Model: "SSD", Size: 1000204886016, InterfaceType: "SCSI"}}
		case *[]networkAdapterWMI:
			*result = []networkAdapterWMI{{Name: "Ethernet", MACAddress: " aa:bb:cc:dd:ee:ff ", PhysicalAdapter: true}, {Name: "Virtual", PhysicalAdapter: false}}
		case *[]systemProductWMI:
			*result = []systemProductWMI{{UUID: "hardware-uuid"}}
		case *[]firmwareWMI:
			*result = []firmwareWMI{{Name: "BIOS", SMBIOSBIOSVersion: "1.2", ReleaseDate: "20220513000000.000000+000"}}
		default:
			return fmt.Errorf("unexpected query %s", query)
		}
		return nil
	}
	d := collectDeviceAttributes()
	if d["total_memory_bytes"].Value != "17179869184" || d["windows_install_date"].Value != "2024-06-25" || d["windows_install_date"].Raw != "20240625120100.000000+180" {
		t.Fatalf("device: %+v", d)
	}
	if d["windows_display_version"].Value != "23H2" || d["windows_build"].Value != "22631" {
		t.Fatalf("Windows version/build: %+v", d)
	}
	cpus := collectProcessors()
	if len(cpus) != 1 || cpus[0].Attributes["cores"].Value != "4" || cpus[0].Attributes["logical_processors"].Value != "8" {
		t.Fatalf("cpus: %+v", cpus)
	}
	disks := collectDiskDrives()
	if len(disks) != 1 || disks[0].Attributes["capacity_bytes"].Value != "1000204886016" {
		t.Fatalf("disks: %+v", disks)
	}
	adapters := collectNetworkAdapters()
	if len(adapters) != 1 || adapters[0].Attributes["mac_address"].Value != "AA:BB:CC:DD:EE:FF" || adapters[0].Attributes["mac_address"].Raw != " aa:bb:cc:dd:ee:ff " {
		t.Fatalf("adapters: %+v", adapters)
	}
	bios := collectFirmware()
	if len(bios) != 1 || bios[0].Attributes["release_date"].Value != "2022-05-13" {
		t.Fatalf("bios: %+v", bios)
	}
}

func TestInvalidWMIDateIsPreserved(t *testing.T) {
	v := wmiDate("unknown-date", "WMI")
	if v.Status != "unavailable" || len(v.Observations) != 1 || v.Observations[0].Raw != "unknown-date" {
		t.Fatalf("date: %+v", v)
	}
}

func TestDateRegistryFallbackAndProductID(t *testing.T) {
	previous, oldDate, oldRegistry := queryInventory, readInstallDate, readRegistryString
	t.Cleanup(func() { queryInventory, readInstallDate, readRegistryString = previous, oldDate, oldRegistry })
	readInstallDate = func() report.Observation {
		return report.Observation{Source: "registry", Status: "error", Raw: "access denied"}
	}
	readRegistryString = func(path, name string) report.Observation {
		if name == "MachineId" {
			return report.Observation{Source: "registry device", Status: "collected", Value: "{ABC-123}", Raw: "{ABC-123}"}
		}
		return report.Observation{Source: "registry product", Status: "unavailable"}
	}
	queryInventory = func(query string, dst interface{}) error {
		switch v := dst.(type) {
		case *[]osDetailsWMI:
			*v = []osDetailsWMI{{InstallDate: "20240625120100.000000+180", SerialNumber: "product-id"}}
		case *[]computerMemoryWMI:
			*v = []computerMemoryWMI{{}}
		case *[]systemProductWMI:
			*v = []systemProductWMI{{}}
		}
		return nil
	}
	d := collectDeviceAttributes()
	if d["windows_install_date"].Value != "2024-06-25" || len(d["windows_install_date"].Observations) != 2 || d["windows_install_date"].Observations[0].Status != "error" {
		t.Fatalf("date: %+v", d["windows_install_date"])
	}
	if d["windows_device_id"].Value != "ABC-123" || d["windows_product_id"].Value != "product-id" {
		t.Fatalf("ids: %+v", d)
	}
}

func TestInstalledMemoryFallback(t *testing.T) {
	previous := installedMemoryObservation
	t.Cleanup(func() { installedMemoryObservation = previous })
	installedMemoryObservation = func() report.Observation { return report.Observation{Source: "api", Status: "error"} }
	modules := []report.Component{{Attributes: map[string]report.Value{"capacity_bytes": {Value: "4294967296", Status: "collected"}}}}
	if v := collectInstalledMemory(modules); v.Value != "4294967296" || v.Source != "WMI Win32_PhysicalMemory.Capacity (сумма модулей)" {
		t.Fatalf("fallback: %+v", v)
	}
}

func TestVolumeSpaceZeroAndNil(t *testing.T) {
	previous := queryPointerInventory
	t.Cleanup(func() { queryPointerInventory = previous })
	zero, size := uint64(0), uint64(100)
	queryPointerInventory = func(_ string, dst interface{}) error {
		*(dst.(*[]logicalVolumeWMI)) = []logicalVolumeWMI{{DeviceID: "C:", DriveType: 3, Size: &size, FreeSpace: &zero}, {DeviceID: "D:", DriveType: 2, Size: nil, FreeSpace: nil}, {DeviceID: "Z:", DriveType: 4}}
		return nil
	}
	v := collectLogicalVolumes()
	if len(v) != 2 || v[0].Attributes["free_bytes"].Value != "0" || v[0].Attributes["free_bytes"].Status != "collected" || v[1].Attributes["capacity_bytes"].Status != "unavailable" {
		t.Fatalf("volumes: %+v", v)
	}
}

func TestPeripheralTypesAndIndependentFailures(t *testing.T) {
	old := queryInventory
	t.Cleanup(func() { queryInventory = old })
	queryInventory = func(query string, dst interface{}) error {
		switch d := dst.(type) {
		case *[]monitorWMI:
			*d = []monitorWMI{{Name: "Display", MonitorManufacturer: "Vendor", PNPDeviceID: "MONITOR\\ABC", ScreenWidth: 1920, ScreenHeight: 1080}}
		case *[]peripheralWMI:
			if strings.Contains(query, "Win32_Keyboard") {
				return fmt.Errorf("keyboard error")
			}
			*d = []peripheralWMI{{Name: "Mouse"}}
		case *[]printerWMI:
			*d = []printerWMI{{Name: "Print to PDF", PortName: "PORTPROMPT:"}}
		default:
			return fmt.Errorf("unexpected query %s", query)
		}
		return nil
	}
	parts := collectPeripherals()
	if len(parts) != 4 {
		t.Fatalf("peripherals: %+v", parts)
	}
	if parts[0].Manufacturer.Value != "Vendor" || parts[0].Attributes["screen_width"].Value != "1920" || parts[1].Name.Observations[0].Status != "error" || parts[2].Name.Value != "Mouse" || parts[3].Name.Value != "Print to PDF" {
		t.Fatalf("peripherals: %+v", parts)
	}
}

func TestUSBInventoryAndDiskIDs(t *testing.T) {
	previous := queryInventory
	t.Cleanup(func() { queryInventory = previous })
	queryInventory = func(query string, dst interface{}) error {
		switch v := dst.(type) {
		case *[]usbDeviceWMI:
			*v = []usbDeviceWMI{{Name: "USB camera", PNPDeviceID: `USB\VID_1234`, PNPClass: "Image"}, {Name: "Flash", PNPDeviceID: `USB\VID_9876`, Service: "USBSTOR"}, {Name: "Disk", PNPDeviceID: `USBSTOR\Disk`, PNPClass: "DiskDrive"}}
		case *[]diskDriveWMI:
			*v = []diskDriveWMI{{Model: "Flash", InterfaceType: "USB", PNPDeviceID: `USBSTOR\Disk`}}
		default:
			return fmt.Errorf("unexpected: %s", query)
		}
		return nil
	}
	usb := collectUSBDevices()
	if len(usb) != 1 || usb[0].Name.Value != "USB camera" || usb[0].Attributes["pnp_device_id"].Value != `USB\VID_1234` {
		t.Fatalf("usb: %+v", usb)
	}
	disks := collectDiskDrives()
	if len(disks) != 1 || disks[0].Attributes["pnp_device_id"].Value != `USBSTOR\Disk` {
		t.Fatalf("disk: %+v", disks)
	}
}
