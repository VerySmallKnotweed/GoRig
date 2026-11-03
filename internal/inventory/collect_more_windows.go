//go:build windows

package inventory

import (
	"net"
	"strings"
	"time"

	"GoRig/internal/report"
)

type osDetailsWMI struct {
	Caption        string
	Version        string
	OSArchitecture string
	InstallDate    string
	SerialNumber   string
}
type computerMemoryWMI struct {
	TotalPhysicalMemory uint64
	SystemType          string
}
type networkAdapterWMI struct {
	Name            string
	Manufacturer    string
	MACAddress      string
	PNPDeviceID     string
	PhysicalAdapter bool
}
type firmwareWMI struct {
	Name              string
	Manufacturer      string
	SerialNumber      string
	SMBIOSBIOSVersion string
	ReleaseDate       string
}

func missingQuery(source string, err error) report.Value {
	return queryFailure("", source, err)[0].Name
}

// WMI uses CIM_DATETIME (YYYYMMDDhhmmss...); show the date and keep raw input.
func wmiDate(raw, source string) report.Value {
	if len(raw) < 8 {
		return selectPrimaryValue([]report.Observation{{Source: source, Status: "unavailable", Raw: raw}})
	}
	date, err := time.Parse("20060102", raw[:8])
	if err != nil {
		return selectPrimaryValue([]report.Observation{{Source: source, Status: "unavailable", Raw: raw}})
	}
	o := makeObservation(date.Format("2006-01-02"), source)
	o.Raw = raw
	return selectPrimaryValue([]report.Observation{o})
}

func collectDeviceAttributes() map[string]report.Value {
	result := make(map[string]report.Value)
	var systems []computerMemoryWMI
	err := queryInventory("SELECT TotalPhysicalMemory, SystemType FROM Win32_ComputerSystem", &systems)
	if err != nil || len(systems) == 0 {
		result["total_memory_bytes"] = missingQuery("WMI Win32_ComputerSystem.TotalPhysicalMemory", err)
		result["system_type"] = missingQuery("WMI Win32_ComputerSystem.SystemType", err)
	} else {
		result["total_memory_bytes"] = wmiNumber(systems[0].TotalPhysicalMemory, "WMI Win32_ComputerSystem.TotalPhysicalMemory")
		result["system_type"] = wmiString(systems[0].SystemType, "WMI Win32_ComputerSystem.SystemType")
	}
	var osInfo []osDetailsWMI
	err = queryInventory("SELECT Caption, Version, OSArchitecture, InstallDate, SerialNumber FROM Win32_OperatingSystem", &osInfo)
	if err != nil || len(osInfo) == 0 {
		for _, key := range []string{"windows_name", "windows_version", "windows_architecture", "windows_install_date", "windows_product_id"} {
			result[key] = missingQuery("WMI Win32_OperatingSystem", err)
		}
	} else {
		os := osInfo[0]
		result["windows_name"] = wmiString(os.Caption, "WMI Win32_OperatingSystem.Caption")
		result["windows_version"] = wmiString(os.Version, "WMI Win32_OperatingSystem.Version")
		result["windows_architecture"] = wmiString(os.OSArchitecture, "WMI Win32_OperatingSystem.OSArchitecture")
		result["windows_install_date"] = wmiDate(os.InstallDate, "WMI Win32_OperatingSystem.InstallDate")
		result["windows_product_id"] = wmiString(os.SerialNumber, "WMI Win32_OperatingSystem.SerialNumber")
	}
	result["windows_install_date"] = selectPrimaryValue(append([]report.Observation{readInstallDate()}, result["windows_install_date"].Observations...))
	result["windows_product_id"] = selectPrimaryValue(append([]report.Observation{readRegistryString(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductId")}, result["windows_product_id"].Observations...))
	deviceID := readRegistryString(`SOFTWARE\Microsoft\SQMClient`, "MachineId")
	if deviceID.Status == "collected" {
		deviceID.Value = strings.Trim(strings.TrimSpace(deviceID.Value), "{}")
	}
	result["windows_device_id"] = selectPrimaryValue([]report.Observation{deviceID})
	var products []systemProductWMI
	err = queryInventory("SELECT UUID FROM Win32_ComputerSystemProduct", &products)
	if err != nil || len(products) == 0 {
		result["hardware_uuid"] = missingQuery("WMI Win32_ComputerSystemProduct.UUID", err)
	} else {
		result["hardware_uuid"] = wmiString(products[0].UUID, "WMI Win32_ComputerSystemProduct.UUID")
	}
	result["touch_support"] = collectTouch()
	return result
}

func collectNetworkAdapters() []report.Component {
	var adapters []networkAdapterWMI
	err := queryInventory("SELECT Name, Manufacturer, MACAddress, PNPDeviceID, PhysicalAdapter FROM Win32_NetworkAdapter", &adapters)
	if err != nil {
		return queryFailure("network", "WMI Win32_NetworkAdapter", err)
	}
	var components []report.Component
	interfaces, _ := net.Interfaces()
	ips := ipAddressesByMAC(interfaces, func(i net.Interface) ([]net.Addr, error) { return i.Addrs() })
	for _, adapter := range adapters {
		if !adapter.PhysicalAdapter {
			continue
		}
		attributes := map[string]report.Value{
			"mac_address":   wmiNormalizedMAC(adapter.MACAddress),
			"pnp_device_id": wmiString(adapter.PNPDeviceID, "WMI Win32_NetworkAdapter.PNPDeviceID"),
		}
		if addresses := ips[strings.ToUpper(strings.TrimSpace(adapter.MACAddress))]; len(addresses) > 0 {
			attributes["ip_addresses"] = wmiString(strings.Join(addresses, ", "), "net.Interface.Addrs")
		}
		components = append(components, report.Component{
			Type: "network", Name: wmiString(adapter.Name, "WMI Win32_NetworkAdapter.Name"),
			Manufacturer: wmiString(adapter.Manufacturer, "WMI Win32_NetworkAdapter.Manufacturer"),
			Model:        report.Value{Status: "not_collected"}, SerialNumber: report.Value{Status: "not_collected"},
			Attributes: attributes,
		})
	}
	return components
}

func collectFirmware() []report.Component {
	var firmware []firmwareWMI
	err := queryInventory("SELECT Name, Manufacturer, SerialNumber, SMBIOSBIOSVersion, ReleaseDate FROM Win32_BIOS", &firmware)
	if err != nil || len(firmware) == 0 {
		return queryFailure("bios", "WMI Win32_BIOS", err)
	}
	out := make([]report.Component, 0, len(firmware))
	for _, b := range firmware {
		out = append(out, report.Component{
			Type: "bios", Name: wmiString(b.Name, "WMI Win32_BIOS.Name"),
			Manufacturer: wmiString(b.Manufacturer, "WMI Win32_BIOS.Manufacturer"),
			Model:        wmiString(b.SMBIOSBIOSVersion, "WMI Win32_BIOS.SMBIOSBIOSVersion"),
			SerialNumber: wmiString(b.SerialNumber, "WMI Win32_BIOS.SerialNumber"),
			Attributes:   map[string]report.Value{"release_date": wmiDate(b.ReleaseDate, "WMI Win32_BIOS.ReleaseDate")},
		})
	}
	return out
}

func wmiNormalizedMAC(raw string) report.Value {
	o := makeObservation(raw, "WMI Win32_NetworkAdapter.MACAddress")
	o.Value = strings.ToUpper(strings.TrimSpace(raw))
	return selectPrimaryValue([]report.Observation{o})
}
