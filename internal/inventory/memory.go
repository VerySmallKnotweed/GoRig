package inventory

import (
	"fmt"
	"math"
	"strconv"

	"GoRig/internal/report"
)

// sumModules is a fallback only when every reported memory module has a valid,
// nonzero capacity. A partial sum must not be presented as installed memory.
func sumModules(modules []report.Component) report.Observation {
	obs := report.Observation{Source: "WMI Win32_PhysicalMemory.Capacity (сумма модулей)", Status: "unavailable"}
	if len(modules) == 0 {
		obs.Raw = "модули не найдены"
		return obs
	}
	var sum uint64
	for _, module := range modules {
		v, ok := module.Attributes["capacity_bytes"]
		if !ok || v.Status != "collected" {
			obs.Raw = "объём одного или нескольких модулей недоступен"
			return obs
		}
		n, err := strconv.ParseUint(v.Value, 10, 64)
		if err != nil || n == 0 || n > math.MaxUint64-sum {
			obs.Raw = "некорректный объём одного или нескольких модулей"
			return obs
		}
		sum += n
	}
	obs.Status = "collected"
	obs.Value = strconv.FormatUint(sum, 10)
	obs.Raw = fmt.Sprintf("%d байт; модулей: %d", sum, len(modules))
	return obs
}
