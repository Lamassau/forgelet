package cli

import (
	"fmt"
	"os"
)

func isTTY() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func step(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "→ "+format+"\n", a...)
}

func success(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "✓ "+format+"\n", a...)
}

func warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "⚠ "+format+"\n", a...)
}

func printError(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", a...)
}
