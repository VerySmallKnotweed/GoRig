package report

import (
	"encoding/json"
	"os"
)

// WriteFile сохраняет прежнее поведение для явно указанного пути.
func WriteFile(path string, report Report) error {
	return writeFile(path, report, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
}

// WriteNewFile не перезаписывает ранее созданный отчёт.
func WriteNewFile(path string, report Report) error {
	return writeFile(path, report, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
}

func writeFile(path string, report Report, flags int) error {
	file, err := os.OpenFile(path, flags, 0666)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
