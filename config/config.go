package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AlarmConfig struct {
	Prayers     []string `json:"prayers"`
	Azan        bool     `json:"azan"`
	Notify      bool     `json:"notify"`
	ImsakOffset int      `json:"imsakOffset"`
}

type Config struct {
	Alarm AlarmConfig `json:"alarm"`
}

func Default() Config {
	return Config{
		Alarm: AlarmConfig{
			Prayers:     []string{"Fajr", "Dhuhr", "Maghrib"},
			Azan:        true,
			Notify:      true,
			ImsakOffset: 0,
		},
	}
}

func path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "when")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func DataDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cacheDir, "when")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func Load() (Config, error) {
	p, err := path()
	if err != nil {
		return Default(), nil
	}

	f, err := os.Open(p)
	if err != nil {
		return Default(), nil
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return Default(), nil
	}

	if cfg.Alarm.Prayers == nil {
		cfg.Alarm.Prayers = Default().Alarm.Prayers
	}

	return cfg, nil
}
