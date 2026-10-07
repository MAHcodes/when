package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if len(cfg.Alarm.Prayers) != 3 {
		t.Fatalf("expected 3 default prayers, got %d", len(cfg.Alarm.Prayers))
	}

	expected := []string{"Fajr", "Dhuhr", "Maghrib"}
	for i, p := range expected {
		if cfg.Alarm.Prayers[i] != p {
			t.Errorf("prayer %d: expected %q, got %q", i, p, cfg.Alarm.Prayers[i])
		}
	}

	if !cfg.Alarm.Azan {
		t.Error("expected Azan to be true by default")
	}
	if !cfg.Alarm.Notify {
		t.Error("expected Notify to be true by default")
	}
	if cfg.Alarm.ImsakOffset != 0 {
		t.Errorf("expected ImsakOffset 0, got %d", cfg.Alarm.ImsakOffset)
	}
}

func TestLoadDefaultsWhenNoConfig(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("XDG_CONFIG_HOME", oldXDG)
	})

	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	os.Unsetenv("HOME")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Alarm.Prayers) != 3 {
		t.Errorf("expected 3 default prayers, got %d", len(cfg.Alarm.Prayers))
	}
	if !cfg.Alarm.Azan {
		t.Error("expected Azan to be true by default")
	}
}

func TestLoadFromConfigFile(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("XDG_CONFIG_HOME", oldXDG)
	})

	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	os.Unsetenv("HOME")

	cfgDir := filepath.Join(tmp, "when")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgContent := `{"alarm":{"prayers":["Fajr","Maghrib"],"azan":false,"notify":true,"imsakOffset":10}}`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Alarm.Prayers) != 2 {
		t.Errorf("expected 2 prayers, got %d", len(cfg.Alarm.Prayers))
	}
	if cfg.Alarm.Azan {
		t.Error("expected Azan to be false")
	}
	if !cfg.Alarm.Notify {
		t.Error("expected Notify to be true")
	}
	if cfg.Alarm.ImsakOffset != 10 {
		t.Errorf("expected ImsakOffset 10, got %d", cfg.Alarm.ImsakOffset)
	}
}

func TestLoadDefaultsForMissingPrayersField(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("XDG_CONFIG_HOME", oldXDG)
	})

	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	os.Unsetenv("HOME")

	cfgDir := filepath.Join(tmp, "when")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgContent := `{"alarm":{"azan":true,"notify":false}}`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Alarm.Prayers) != 3 {
		t.Errorf("expected default 3 prayers when not specified, got %d", len(cfg.Alarm.Prayers))
	}
	if cfg.Alarm.Notify {
		t.Error("expected Notify to be false")
	}
}

func TestLoadFlatConfig(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("XDG_CONFIG_HOME", oldXDG)
	})

	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	os.Unsetenv("HOME")

	cfgDir := filepath.Join(tmp, "when")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgContent := `{"prayers":["Fajr","Maghrib"],"azan":false,"notify":true,"imsakOffset":10}`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Alarm.Prayers) != 2 {
		t.Errorf("expected 2 prayers, got %d", len(cfg.Alarm.Prayers))
	}
	if cfg.Alarm.Azan {
		t.Error("expected Azan to be false")
	}
	if !cfg.Alarm.Notify {
		t.Error("expected Notify to be true")
	}
	if cfg.Alarm.ImsakOffset != 10 {
		t.Errorf("expected ImsakOffset 10, got %d", cfg.Alarm.ImsakOffset)
	}
}

func TestLoadNestedSectionWinsOverFlatKeys(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("XDG_CONFIG_HOME", oldXDG)
	})

	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	os.Unsetenv("HOME")

	cfgDir := filepath.Join(tmp, "when")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgContent := `{"notify":true,"alarm":{"azan":true,"notify":false}}`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.Alarm.Azan {
		t.Error("expected Azan to be true")
	}
	if cfg.Alarm.Notify {
		t.Error("expected Notify to be false: nested \"alarm\" section must win")
	}
}

func TestLoadIgnoresUnknownKeys(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("XDG_CONFIG_HOME", oldXDG)
	})

	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	os.Unsetenv("HOME")

	cfgDir := filepath.Join(tmp, "when")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfgContent := `{"alarm":{"azan":true,"notify":true,"loud":true},"version":2}`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.Alarm.Azan || !cfg.Alarm.Notify {
		t.Errorf("expected azan and notify to stay true, got azan=%v notify=%v", cfg.Alarm.Azan, cfg.Alarm.Notify)
	}
	if len(cfg.Alarm.Prayers) != 3 {
		t.Errorf("expected default prayers, got %d", len(cfg.Alarm.Prayers))
	}
}

func TestDataDir(t *testing.T) {
	oldCache := os.Getenv("XDG_CACHE_HOME")
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		os.Setenv("XDG_CACHE_HOME", oldCache)
		os.Setenv("HOME", oldHome)
	})

	tmp := t.TempDir()
	os.Setenv("XDG_CACHE_HOME", tmp)
	os.Unsetenv("HOME")

	dir, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(tmp, "when")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("DataDir did not create the directory")
	}
}

func TestLoadHandlesInvalidJSON(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("XDG_CONFIG_HOME", oldXDG)
	})

	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	os.Unsetenv("HOME")

	cfgDir := filepath.Join(tmp, "when")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Alarm.Prayers) != 3 {
		t.Errorf("expected default config on invalid JSON, got %d prayers", len(cfg.Alarm.Prayers))
	}
}
