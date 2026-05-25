package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{name: "empty slice", values: nil, want: ""},
		{name: "all empty", values: []string{"", "   ", "\t"}, want: ""},
		{name: "first non-empty", values: []string{"alpha", "beta"}, want: "alpha"},
		{name: "middle non-empty", values: []string{"", " beta ", "gamma"}, want: " beta "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstNonEmpty(tt.values...); got != tt.want {
				t.Fatalf("firstNonEmpty() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveVarRef(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		appName     string
		serviceName string
		want        string
	}{
		{name: "app name", value: "${app.name}", appName: "forgelet", want: "forgelet"},
		{name: "service name", value: "${service.name}", appName: "forgelet", serviceName: "api", want: "api"},
		{name: "both", value: "${app.name}-${service.name}", appName: "forgelet", serviceName: "web", want: "forgelet-web"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVarRef(tt.value, tt.appName, tt.serviceName); got != tt.want {
				t.Fatalf("resolveVarRef() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateMetalLBPool(t *testing.T) {
	tests := []struct {
		name    string
		pool    string
		wantErr bool
	}{
		{name: "valid range", pool: "192.168.1.10-192.168.1.20", wantErr: false},
		{name: "missing dash", pool: "192.168.1.10", wantErr: true},
		{name: "invalid start ip", pool: "bad-ip-192.168.1.20", wantErr: true},
		{name: "invalid end ip", pool: "192.168.1.10-bad-ip", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMetalLBPool(tt.pool)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateMetalLBPool() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildK0SConfig(t *testing.T) {
	configYAML, err := buildK0SConfig("dev-cluster", "192.168.64.10")
	if err != nil {
		t.Fatalf("buildK0SConfig() error = %v", err)
	}

	var cfg map[string]any
	if err := yaml.Unmarshal([]byte(configYAML), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if got := cfg["apiVersion"]; got != "k0s.k0sproject.io/v1beta1" {
		t.Fatalf("apiVersion = %v, want %q", got, "k0s.k0sproject.io/v1beta1")
	}
	if got := cfg["kind"]; got != "ClusterConfig" {
		t.Fatalf("kind = %v, want %q", got, "ClusterConfig")
	}

	spec, ok := cfg["spec"].(map[string]any)
	if !ok {
		t.Fatalf("spec not found in config")
	}
	api, ok := spec["api"].(map[string]any)
	if !ok {
		t.Fatalf("api block not found in config")
	}
	sans, ok := api["sans"].([]any)
	if !ok {
		t.Fatalf("sans not found in config")
	}

	found := false
	for _, san := range sans {
		if san == "192.168.64.10" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected SANs to contain host IP, got %#v", sans)
	}
}

func TestParseComposeBuildServices(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")
	compose := `services:
  api:
    build: .
  web:
    build:
      context: ./web
      dockerfile: Dockerfile.dev
  worker:
    image: busybox
`
	if err := os.WriteFile(composePath, []byte(compose), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	services, err := parseComposeBuildServices(composePath, "demo")
	if err != nil {
		t.Fatalf("parseComposeBuildServices() error = %v", err)
	}

	want := []BuildService{
		{Name: "api", Image: "demo-api", Description: "api", Dockerfile: "Dockerfile", Context: ".", DevTarget: "dev", ProdTarget: "prod"},
		{Name: "web", Image: "demo-web", Description: "web", Dockerfile: "Dockerfile.dev", Context: "./web", DevTarget: "dev", ProdTarget: "prod"},
	}
	if !reflect.DeepEqual(services, want) {
		t.Fatalf("parseComposeBuildServices() = %#v, want %#v", services, want)
	}
}

func TestUpdateHostsEntries(t *testing.T) {
	hostsPath := filepath.Join(t.TempDir(), "hosts")
	initial := strings.Join([]string{
		"127.0.0.1 localhost",
		"10.0.0.1 old.example.local # k0s-test",
		"10.0.0.1 api.old.example.local # k0s-test",
		"",
	}, "\n")
	if err := os.WriteFile(hostsPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	entries := []string{
		"10.0.0.2 example.local # k0s-test",
		"10.0.0.2 api.example.local # k0s-test",
	}
	if err := updateHostsEntries(hostsPath, "# k0s-test", entries, false); err != nil {
		t.Fatalf("updateHostsEntries() error = %v", err)
	}

	data, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	got := string(data)
	if strings.Contains(got, "old.example.local") {
		t.Fatalf("expected old marker lines to be removed, got %q", got)
	}
	for _, entry := range entries {
		if !strings.Contains(got, entry) {
			t.Fatalf("expected hosts file to contain %q, got %q", entry, got)
		}
	}
}
