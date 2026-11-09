package config

import (
	"encoding/json"
	"fmt"
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

func alarmFields() map[string]bool {
	return map[string]bool{
		"prayers":     true,
		"azan":        true,
		"notify":      true,
		"imsakOffset": true,
	}
}

func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "when: warning: "+format+"\n", args...)
}

func Load() (Config, error) {
	p, err := path()
	if err != nil {
		return Default(), nil
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return Default(), nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		warnf("%s is not valid JSON, using defaults: %v", p, err)
		return Default(), nil
	}

	cfg := Config{Alarm: AlarmConfig{}}
	fields := alarmFields()

	section, hasSection := raw["alarm"]
	switch {
	case hasSection:
		for k := range raw {
			if fields[k] {
				warnf("%s: top-level %q is ignored, move it into the \"alarm\" object", p, k)
			}
		}
	default:
		flat := map[string]json.RawMessage{}
		for k, v := range raw {
			if fields[k] {
				flat[k] = v
			}
		}
		if len(flat) > 0 {
			section, _ = json.Marshal(flat)
			hasSection = true
		}
	}

	if hasSection {
		var alarmCfg AlarmConfig
		if err := json.Unmarshal(section, &alarmCfg); err != nil {
			warnf("%s: invalid \"alarm\" settings, ignoring them: %v", p, err)
		} else {
			cfg.Alarm = alarmCfg
		}

		var sectionKeys map[string]json.RawMessage
		if err := json.Unmarshal(section, &sectionKeys); err == nil {
			for k := range sectionKeys {
				if !fields[k] {
					warnf("%s: unknown alarm setting %q", p, k)
				}
			}
		}
	}

	for k := range raw {
		if k != "alarm" && !fields[k] {
			warnf("%s: unknown setting %q", p, k)
		}
	}

	if cfg.Alarm.Prayers == nil {
		cfg.Alarm.Prayers = Default().Alarm.Prayers
	}

	return cfg, nil
}
