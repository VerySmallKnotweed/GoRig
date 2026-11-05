//go:build windows

package inventory

import (
	"fmt"
	"math"
	"strconv"
	"time"
	"unsafe"

	"GoRig/internal/report"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type systemProductWMI struct{ UUID string }

var readRegistryString = func(path, name string) report.Observation {
	source := `HKLM\` + path + `\` + name
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return report.Observation{Source: source, Status: "error", Raw: err.Error()}
	}
	defer key.Close()
	v, _, err := key.GetStringValue(name)
	if err != nil {
		return report.Observation{Source: source, Status: "unavailable", Raw: err.Error()}
	}
	return makeObservation(v, source)
}

var readInstallDate = func() report.Observation {
	path := `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
	source := `HKLM\` + path + `\InstallDate`
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return report.Observation{Source: source, Status: "error", Raw: err.Error()}
	}
	defer key.Close()
	seconds, _, err := key.GetIntegerValue("InstallDate")
	if err != nil {
		return report.Observation{Source: source, Status: "unavailable", Raw: err.Error()}
	}
	// InstallDate is a Unix timestamp, not a Windows FILETIME.
	if seconds == 0 || seconds > 4102444800 {
		return report.Observation{Source: source, Status: "unavailable", Raw: strconv.FormatUint(seconds, 10)}
	}
	return report.Observation{Source: source, Status: "collected", Value: time.Unix(int64(seconds), 0).In(time.Local).Format("2006-01-02"), Raw: strconv.FormatUint(seconds, 10)}
}

var installedMemoryObservation = func() report.Observation {
	source := "Windows GetPhysicallyInstalledSystemMemory"
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetPhysicallyInstalledSystemMemory")
	if err := proc.Find(); err != nil {
		return report.Observation{Source: source, Status: "error", Raw: err.Error()}
	}
	var kb uint64
	ok, _, callErr := proc.Call(uintptr(unsafe.Pointer(&kb)))
	if ok == 0 {
		return report.Observation{Source: source, Status: "error", Raw: callErr.Error()}
	}
	if kb == 0 || kb > math.MaxUint64/1024 {
		return report.Observation{Source: source, Status: "unavailable", Raw: fmt.Sprintf("%d KiB", kb)}
	}
	return report.Observation{Source: source, Status: "collected", Value: strconv.FormatUint(kb*1024, 10), Raw: fmt.Sprintf("%d KiB", kb)}
}

func collectInstalledMemory(modules []report.Component) report.Value {
	return selectPrimaryValue([]report.Observation{installedMemoryObservation(), sumModules(modules)})
}

func collectTouch() report.Value {
	proc := windows.NewLazySystemDLL("user32.dll").NewProc("GetSystemMetrics")
	if err := proc.Find(); err != nil {
		return selectPrimaryValue([]report.Observation{{Source: "Windows GetSystemMetrics(SM_DIGITIZER)", Status: "error", Raw: err.Error()}})
	}
	flags, _, _ := proc.Call(94) // SM_DIGITIZER
	value := "Перо и сенсорный ввод не обнаружены"
	switch {
	case flags&3 != 0 && flags&12 != 0:
		value = "Перо и сенсорный ввод"
	case flags&3 != 0:
		value = "Сенсорный ввод"
	case flags&12 != 0:
		value = "Перо"
	}
	return selectPrimaryValue([]report.Observation{{Source: "Windows GetSystemMetrics(SM_DIGITIZER)", Status: "collected", Value: value, Raw: fmt.Sprintf("0x%x", flags)}})
}
