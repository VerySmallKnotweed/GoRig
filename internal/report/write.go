package report

import (
	"encoding/json"
	"os"
)

func WriteFile(path string, report Report) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	return encoder.Encode(report)
}
