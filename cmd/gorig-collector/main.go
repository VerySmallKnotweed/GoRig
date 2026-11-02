package main

import (
	"fmt"
	"os"
	"path/filepath"

	"GoRig/internal/inventory"
	"GoRig/internal/report"
)

const collectorVersion = "0.4.0"

func main() {
	result, err := inventory.Collect(collectorVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось собрать сведения: %v\n", err)
		os.Exit(1)
	}

	var outputPath string
	var writeErr error
	if len(os.Args) > 1 {
		// Явный путь сохраняет прежнее поведение: существующий файл заменяется.
		outputPath = os.Args[1]
		writeErr = report.WriteFile(outputPath, result)
	} else {
		// Храним отчёты рядом с программой, независимо от текущего каталога.
		executable, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Не удалось определить каталог программы: %v\n", err)
			os.Exit(1)
		}
		outputPath = filepath.Join(filepath.Dir(executable), "gorig-report-"+result.ScanID+".json")
		writeErr = report.WriteNewFile(outputPath, result)
	}
	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "Не удалось записать файл %q: %v\n", outputPath, writeErr)
		os.Exit(1)
	}

	fmt.Printf("Файл обследования создан: %s\n", outputPath)
}
