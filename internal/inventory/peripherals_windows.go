//go:build windows

package inventory

import (
	"strconv"

	"GoRig/internal/report"
	"github.com/yusufpapurcu/wmi"
)

type logicalVolumeWMI struct {
	DeviceID   string
	VolumeName string
	FileSystem string
	DriveType  uint32
	Size       *uint64
	FreeSpace  *uint64
}
type peripheralWMI struct {
	Name         string
	Manufacturer string
	PNPDeviceID  string
}
type monitorWMI struct {
	Name                string
	MonitorManufacturer string
	PNPDeviceID         string
	ScreenWidth         uint32
	ScreenHeight        uint32
}
type printerWMI struct {
	Name       string
	DriverName string
	PortName   string
}

var queryPointerInventory = func(query string, dst interface{}) error {
	return (&wmi.Client{NonePtrZero: true, PtrNil: true}).Query(query, dst)
}

func numberPointer(value *uint64, source string) report.Value {
	if value == nil {
		return wmiString("", source)
	}
	return wmiString(strconv.FormatUint(*value, 10), source)
}

func collectLogicalVolumes() []report.Component {
	var volumes []logicalVolumeWMI
	err := queryPointerInventory("SELECT DeviceID, VolumeName, FileSystem, DriveType, Size, FreeSpace FROM Win32_LogicalDisk", &volumes)
	if err != nil {
		return queryFailure("volume", "WMI Win32_LogicalDisk", err)
	}
	var out []report.Component
	for _, vol := range volumes {
		// Fixed and removable local volumes; network drives and optical devices are not physical disks.
		if vol.DriveType != 2 && vol.DriveType != 3 {
			continue
		}
		kind := "Локальный"
		if vol.DriveType == 2 {
			kind = "Съёмный"
		}
		out = append(out, report.Component{
			Type: "volume", Name: wmiString(vol.DeviceID, "WMI Win32_LogicalDisk.DeviceID"),
			Model: wmiString(vol.VolumeName, "WMI Win32_LogicalDisk.VolumeName"), Manufacturer: report.Value{Status: "not_collected"}, SerialNumber: report.Value{Status: "not_collected"},
			Attributes: map[string]report.Value{
				"capacity_bytes": numberPointer(vol.Size, "WMI Win32_LogicalDisk.Size"),
				"free_bytes":     numberPointer(vol.FreeSpace, "WMI Win32_LogicalDisk.FreeSpace"),
				"filesystem":     wmiString(vol.FileSystem, "WMI Win32_LogicalDisk.FileSystem"),
				"volume_type":    wmiString(kind, "WMI Win32_LogicalDisk.DriveType"),
			},
		})
	}
	return out
}

func collectPeripherals() []report.Component {
	var out []report.Component
	var monitors []monitorWMI
	monitorErr := queryInventory("SELECT Name, MonitorManufacturer, PNPDeviceID, ScreenWidth, ScreenHeight FROM Win32_DesktopMonitor", &monitors)
	if monitorErr != nil {
		out = append(out, queryFailure("monitor", "WMI Win32_DesktopMonitor", monitorErr)...)
	}
	for _, m := range monitors {
		out = append(out, report.Component{Type: "monitor", Name: wmiString(m.Name, "WMI Win32_DesktopMonitor.Name"), Manufacturer: wmiString(m.MonitorManufacturer, "WMI Win32_DesktopMonitor.MonitorManufacturer"), Model: report.Value{Status: "not_collected"}, SerialNumber: report.Value{Status: "not_collected"}, Attributes: map[string]report.Value{"pnp_device_id": wmiString(m.PNPDeviceID, "WMI Win32_DesktopMonitor.PNPDeviceID"), "screen_width": wmiNumber(uint64(m.ScreenWidth), "WMI Win32_DesktopMonitor.ScreenWidth"), "screen_height": wmiNumber(uint64(m.ScreenHeight), "WMI Win32_DesktopMonitor.ScreenHeight")}})
	}
	for _, c := range []struct{ class, kind string }{{"Win32_Keyboard", "keyboard"}, {"Win32_PointingDevice", "mouse"}} {
		var devices []peripheralWMI
		err := queryInventory("SELECT Name, Manufacturer, PNPDeviceID FROM "+c.class, &devices)
		if err != nil {
			out = append(out, queryFailure(c.kind, "WMI "+c.class, err)...)
			continue
		}
		for _, d := range devices {
			out = append(out, report.Component{Type: c.kind, Name: wmiString(d.Name, "WMI "+c.class+".Name"), Manufacturer: wmiString(d.Manufacturer, "WMI "+c.class+".Manufacturer"), Model: report.Value{Status: "not_collected"}, SerialNumber: report.Value{Status: "not_collected"}, Attributes: map[string]report.Value{"pnp_device_id": wmiString(d.PNPDeviceID, "WMI "+c.class+".PNPDeviceID")}})
		}
	}
	var printers []printerWMI
	err := queryInventory("SELECT Name, DriverName, PortName FROM Win32_Printer", &printers)
	if err != nil {
		return append(out, queryFailure("printer", "WMI Win32_Printer", err)...)
	}
	for _, p := range printers {
		out = append(out, report.Component{Type: "printer", Name: wmiString(p.Name, "WMI Win32_Printer.Name"), Manufacturer: report.Value{Status: "not_collected"}, Model: report.Value{Status: "not_collected"}, SerialNumber: report.Value{Status: "not_collected"}, Attributes: map[string]report.Value{"printer_driver": wmiString(p.DriverName, "WMI Win32_Printer.DriverName"), "printer_port": wmiString(p.PortName, "WMI Win32_Printer.PortName")}})
	}
	return out
}
