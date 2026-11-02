package main

import (
	"fmt"
	"os"

	"GoRig/internal/inventory"
	"GoRig/internal/report"
)

const collectorVersion = "0.1.0"

func main() {
	outputPath := "gorig-report.json"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}

	result, err := inventory.Collect(collectorVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось собрать сведения: %v\n", err)
		os.Exit(1)
	}

	if err := report.WriteFile(outputPath, result); err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось записать файл %q: %v\n", outputPath, err)
		os.Exit(1)
	}

	fmt.Printf("Файл обследования создан: %s\n", outputPath)
}
