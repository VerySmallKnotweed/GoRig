//go:build !windows

package console

import (
	"fmt"
	"os"
)

func Setup([]string) (bool, error) { return true, nil }
func Error(_ bool, text string)    { fmt.Fprintln(os.Stderr, text) }
func Info(_ bool, text string)     { fmt.Fprintln(os.Stdout, text) }
