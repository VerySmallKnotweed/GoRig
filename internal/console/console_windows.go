//go:build windows

// Package console optionally attaches a diagnostic console to a Windows GUI executable.
package console

import (
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel           = syscall.NewLazyDLL("kernel32.dll")
	attachConsole    = kernel.NewProc("AttachConsole")
	allocConsole     = kernel.NewProc("AllocConsole")
	getConsoleWindow = kernel.NewProc("GetConsoleWindow")
	messageBox       = syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
)

// Setup should be called before logging or parsing flags. GUI builds remain
// silent unless -console/--console is explicitly present.
func Setup(args []string) (bool, error) {
	enabled := false
	for _, arg := range args {
		if arg == "-console" || arg == "--console" || arg == "-console=true" || arg == "--console=true" {
			enabled = true
		}
	}
	if !enabled {
		return false, nil
	}
	if hwnd, _, _ := getConsoleWindow.Call(); hwnd == 0 {
		if ok, _, _ := attachConsole.Call(^uintptr(0)); ok == 0 {
			if ok, _, err := allocConsole.Call(); ok == 0 {
				return true, fmt.Errorf("не удалось открыть консоль: %w", err)
			}
		}
	}
	out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return true, err
	}
	in, err := os.OpenFile("CONIN$", os.O_RDONLY, 0)
	if err != nil {
		out.Close()
		return true, err
	}
	os.Stdout, os.Stderr, os.Stdin = out, out, in
	_ = windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, windows.Handle(out.Fd()))
	_ = windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(out.Fd()))
	_ = windows.SetStdHandle(windows.STD_INPUT_HANDLE, windows.Handle(in.Fd()))
	log.SetOutput(out)
	return true, nil
}
func message(title, text string, flags uintptr) {
	t, err := syscall.UTF16PtrFromString(strings.ReplaceAll(title, "\x00", ""))
	if err != nil {
		return
	}
	s, err := syscall.UTF16PtrFromString(strings.ReplaceAll(text, "\x00", ""))
	if err != nil {
		return
	}
	messageBox.Call(0, uintptr(unsafe.Pointer(s)), uintptr(unsafe.Pointer(t)), flags)
}
func Error(enabled bool, text string) {
	if enabled {
		fmt.Fprintln(os.Stderr, text)
	} else {
		message("GoRig — ошибка", text, 0x10)
	}
}
func Info(enabled bool, text string) {
	if enabled {
		fmt.Fprintln(os.Stdout, text)
	} else {
		message("GoRig", text, 0x40)
	}
}
