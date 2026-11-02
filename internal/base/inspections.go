package base

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"GoRig/internal/report"
)

type Difference struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

func decodeReport(raw []byte) (report.Report, error) {
	var r report.Report
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, fmt.Errorf("неверный JSON: %w", err)
	}
	if r.SchemaVersion != report.SchemaVersion {
		return r, fmt.Errorf("неподдерживаемая версия формата: %d", r.SchemaVersion)
	}
	if strings.TrimSpace(r.ScanID) == "" || len(r.ScanID) > 200 || r.CollectedAt.IsZero() || r.Device.Components == nil {
		return r, fmt.Errorf("в отчёте отсутствует scan_id, collected_at или device.components")
	}
	return r, nil
}

func value(v report.Value) string {
	if v.Status != "collected" && v.Status != "template" {
		return ""
	}
	return strings.TrimSpace(v.Value)
}

// Canonical values deliberately omit source, observations, collector version,
// scan ID and timestamps. These are evidence of a scan, not its configuration.
func summary(r report.Report) map[string]string {
	d := r.Device
	osName := value(d.OS)
	if osName != "" && strings.TrimSpace(d.OS.Raw) != "" {
		osName += " | " + strings.TrimSpace(d.OS.Raw)
	}
	result := map[string]string{
		"Имя компьютера":            value(d.ComputerName),
		"Производитель компьютера":  value(d.Manufacturer),
		"Модель компьютера":         value(d.Model),
		"Серийный номер компьютера": value(d.SerialNumber),
		"Операционная система":      osName,
	}
	for key, attribute := range d.Attributes {
		if v := value(attribute); v != "" {
			result["Компьютер: "+attributeLabel(key)] = v
		}
	}
	groups := map[string][]string{}
	for _, c := range d.Components {
		parts := []string{}
		add := func(label, v string) {
			if v != "" {
				parts = append(parts, label+": "+v)
			}
		}
		add("Название", value(c.Name))
		add("Производитель", value(c.Manufacturer))
		add("Модель", value(c.Model))
		add("Серийный номер", value(c.SerialNumber))
		keys := make([]string, 0, len(c.Attributes))
		for k := range c.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			// Free disk space changes constantly; it is telemetry, not configuration.
			if c.Type == "volume" && k == "free_bytes" {
				continue
			}
			add(attributeLabel(k), value(c.Attributes[k]))
		}
		// An error/no-instance placeholder is not an installed component.
		if len(parts) != 0 {
			groups[c.Type] = append(groups[c.Type], strings.Join(parts, "; "))
		}
	}
	for kind, items := range groups {
		sort.Strings(items)
		result["Компоненты: "+componentLabel(kind)] = strings.Join(items, "\n")
	}
	return result
}

func componentLabel(kind string) string {
	switch kind {
	case "processor":
		return "процессоры"
	case "disk":
		return "накопители"
	case "memory":
		return "память"
	case "motherboard":
		return "материнские платы"
	case "video":
		return "видеоконтроллеры"
	case "network":
		return "сетевые адаптеры"
	case "bios":
		return "BIOS"
	case "volume":
		return "логические тома"
	case "monitor":
		return "мониторы"
	case "keyboard":
		return "клавиатуры"
	case "mouse":
		return "указательные устройства"
	case "printer":
		return "принтеры"
	default:
		return kind
	}
}

func attributeLabel(key string) string {
	switch key {
	case "capacity_bytes":
		return "Объём, байт"
	case "speed_mhz":
		return "Скорость, МГц"
	case "driver_version":
		return "Версия драйвера"
	case "pnp_device_id":
		return "PNP ID"
	case "total_memory_bytes":
		return "Доступная системе физическая память, байт"
	case "installed_memory_bytes":
		return "Установленная физическая память, байт"
	case "windows_device_id":
		return "Код устройства Windows (SQM)"
	case "windows_product_id":
		return "Код продукта Windows"
	case "hardware_uuid":
		return "Аппаратный UUID"
	case "system_type":
		return "Тип процессора и системы"
	case "touch_support":
		return "Перо и сенсорный ввод"
	case "video_memory_bytes":
		return "Видеопамять по данным WMI, байт"
	case "free_bytes":
		return "Свободно на томе, байт"
	case "filesystem":
		return "Файловая система"
	case "volume_type":
		return "Тип тома"
	case "printer_driver":
		return "Драйвер принтера"
	case "printer_port":
		return "Порт принтера"
	case "screen_width":
		return "Ширина экрана, пикселей"
	case "screen_height":
		return "Высота экрана, пикселей"
	case "windows_name":
		return "Редакция Windows"
	case "windows_version":
		return "Версия Windows"
	case "windows_architecture":
		return "Архитектура Windows"
	case "windows_install_date":
		return "Дата установки Windows"
	case "cores":
		return "Физические ядра"
	case "logical_processors":
		return "Логические процессоры"
	case "max_clock_mhz":
		return "Макс. частота, МГц"
	case "interface_type":
		return "Интерфейс"
	case "media_type":
		return "Тип носителя (WMI)"
	case "mac_address":
		return "MAC-адрес"
	case "release_date":
		return "Дата выпуска"
	default:
		return key
	}
}

func compare(before, after report.Report) []Difference {
	a, b := summary(before), summary(after)
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	var sorted []string
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	diffs := []Difference{}
	for _, k := range sorted {
		if a[k] != b[k] {
			diffs = append(diffs, Difference{Field: k, Before: a[k], After: b[k]})
		}
	}
	return diffs
}
