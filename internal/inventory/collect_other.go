//go:build !windows

package inventory

import (
	"runtime"

	"GoRig/internal/report"
)

func collectOS() report.Value {
	osName := runtime.GOOS
	return report.Value{
		Value:  osName,
		Raw:    runtime.GOOS,
		Source: "runtime.GOOS",
		Status: "collected",
	}
}
