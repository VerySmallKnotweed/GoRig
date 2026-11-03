package main

import (
	"fmt"
	"os"
	"path/filepath"

	"GoRig/internal/console"
	"GoRig/internal/inventory"
	"GoRig/internal/report"
)

const collectorVersion = "0.5.0"

func main() {
	diagnostics, err := console.Setup(os.Args[1:])
	if err != nil {
		console.Error(false, err.Error())
		os.Exit(1)
	}
	args := []string{}
	for _, a := range os.Args[1:] {
		if a != "-console" && a != "--console" && a != "-console=true" && a != "--console=true" {
			args = append(args, a)
		}
	}
	if len(args) > 1 {
		console.Error(diagnostics, "Использование: GoRigCollector.exe [--console] [путь к отчёту]")
		os.Exit(1)
	}
	result, err := inventory.Collect(collectorVersion)
	if err != nil {
		console.Error(diagnostics, fmt.Sprintf("Не удалось собрать сведения: %v", err))
		os.Exit(1)
	}

	var outputPath string
	var writeErr error
	if len(args) > 0 {
		// Явный путь сохраняет прежнее поведение: существующий файл заменяется.
		outputPath = args[0]
		writeErr = report.WriteFile(outputPath, result)
	} else {
		// Храним отчёты рядом с программой, независимо от текущего каталога.
		executable, err := os.Executable()
		if err != nil {
			console.Error(diagnostics, fmt.Sprintf("Не удалось определить каталог программы: %v", err))
			os.Exit(1)
		}
		outputPath = filepath.Join(filepath.Dir(executable), "gorig-report-"+result.ScanID+".json")
		writeErr = report.WriteNewFile(outputPath, result)
	}
	if writeErr != nil {
		console.Error(diagnostics, fmt.Sprintf("Не удалось записать файл %q: %v", outputPath, writeErr))
		os.Exit(1)
	}

	console.Info(diagnostics, fmt.Sprintf("Файл обследования создан: %s", outputPath))
}
