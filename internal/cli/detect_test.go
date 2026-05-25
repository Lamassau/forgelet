package cli

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestDetectPlatform(t *testing.T) {
	t.Setenv("CODESPACES", "")
	platform, mode := detectPlatform()
	if platform == "" || mode == "" {
		t.Fatalf("detectPlatform() returned empty values: platform=%q mode=%q", platform, mode)
	}
	if runtime.GOOS != "linux" {
		return
	}
	data, err := os.ReadFile("/proc/version")
	if err == nil {
		lower := strings.ToLower(string(data))
		if !strings.Contains(lower, "microsoft") && !strings.Contains(lower, "wsl") {
			if platform != "linux" || mode != "native" {
				t.Fatalf("detectPlatform() = (%q, %q), want (%q, %q)", platform, mode, "linux", "native")
			}
		}
	}
}
