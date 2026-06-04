package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := Default()
	cfg.Database.Host = "192.168.0.10"
	cfg.Database.PasswordRef = "dpapi:test"
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadOrDefault(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Database.Host != "192.168.0.10" {
		t.Fatalf("host = %q", loaded.Database.Host)
	}
	if loaded.Server.Port != 3987 {
		t.Fatalf("port = %d", loaded.Server.Port)
	}
}
