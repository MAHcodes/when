package alarm

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/MAHcodes/when/config"
	"github.com/MAHcodes/when/prayer"
)

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

//go:embed azan.webm
var azanData []byte

func Play() error {
	tmp, err := os.CreateTemp("", "azan-*.webm")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(azanData); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	tmp.Close()

	players := [][]string{
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", tmpPath},
		{"paplay", tmpPath},
		{"aplay", tmpPath},
	}

	for _, args := range players {
		cmd := exec.Command(args[0], args[1:]...)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no suitable audio player found (tried ffplay, paplay, aplay)")
}

func Notify(name, timeStr string) {
	msg := fmt.Sprintf("It's time for %s prayer (%s)", name, timeStr)
	if err := exec.Command("notify-send", name, msg).Run(); err == nil {
		return
	} else {
		logPrintf("notify-send failed: %v", err)
	}
	script := fmt.Sprintf("display notification %q with title %q", msg, name)
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		logPrintf("osascript failed: %v", err)
	}
}

func Daemonize() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	logFile := filepath.Join(dataDir, "alarm.log")
	pidFile := filepath.Join(dataDir, "when.pid")

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, "alarm")
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return err
	}

	pid := cmd.Process.Pid
	os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
	fmt.Fprintf(f, "[%s] Daemon started (PID %d)\n", time.Now().Format("2006-01-02 15:04:05"), pid)
	fmt.Printf("Daemon started (PID %d) — log: %s\n", pid, logFile)
	return nil
}

func logPrintf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
}

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func Run() error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	cfg, err := config.Load()
	if err == nil {
		msg := fmt.Sprintf("Alarm started — %s (azan: %s, notify: %s",
			strings.Join(cfg.Alarm.Prayers, ", "), onOff(cfg.Alarm.Azan), onOff(cfg.Alarm.Notify))
		if cfg.Alarm.ImsakOffset > 0 {
			msg += fmt.Sprintf(", suhoor %dmin before Fajr", cfg.Alarm.ImsakOffset)
		}
		msg += ")"
		logPrintf(msg)
	}
	if isTerminal() {
		fmt.Println("Press Ctrl+C to stop.")
	}

	alarmSet := make(map[string]bool)
	for _, n := range cfg.Alarm.Prayers {
		alarmSet[n] = true
	}

	for {
		select {
		case <-sigCh:
			logPrintf("Alarm stopped.")
			return nil
		default:
		}

		prayers, err := prayer.Fetch()
		if err != nil {
			logPrintf("Warning: fetch failed: %v", err)
			time.Sleep(5 * time.Minute)
			continue
		}

		allEntries, err := prayer.TodayEntries(prayers)
		if err != nil {
			logPrintf("Warning: no alarm times available, retrying...")
			time.Sleep(1 * time.Minute)
			continue
		}

		type alarmEntry struct {
			label string
			time  time.Time
		}
		var entries []alarmEntry
		now := time.Now()

		for _, e := range allEntries {
			if !alarmSet[e.Prayer.Name] {
				continue
			}
			entries = append(entries, alarmEntry{e.Prayer.Name, e.Time})
			if e.Prayer.Name == "Fajr" && cfg.Alarm.ImsakOffset > 0 {
				suhoor := e.Time.Add(-time.Duration(cfg.Alarm.ImsakOffset) * time.Minute)
				entries = append(entries, alarmEntry{"Suhoor", suhoor})
			}
		}

		if len(entries) == 0 {
			logPrintf("Warning: no alarm times available, retrying...")
			time.Sleep(1 * time.Minute)
			continue
		}

		var next *alarmEntry
		for i := range entries {
			if entries[i].time.After(now) {
				next = &entries[i]
				break
			}
		}

		if next == nil {
			first := entries[0]
			tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, first.time.Hour(), first.time.Minute(), 0, 0, time.Local)
			next = &alarmEntry{first.label, tomorrow}
			logPrintf("All passed — next: %s tomorrow at %s", next.label, next.time.Format("15:04"))
		} else {
			logPrintf("Next alarm: %s at %s", next.label, next.time.Format("15:04"))
		}

		duration := next.time.Sub(now)
		if duration <= 0 {
			time.Sleep(10 * time.Second)
			continue
		}

		select {
		case <-sigCh:
			logPrintf("Alarm stopped.")
			return nil
		case <-time.After(duration):
			label := next.label
			timeStr := next.time.Format("15:04")

			if label == "Suhoor" {
				logPrintf("%s — time for suhoor", timeStr)
				if cfg.Alarm.Notify {
					logPrintf("%s — sending notification", label)
					go Notify("Suhoor", timeStr)
				}
			} else {
				if cfg.Alarm.Notify {
					logPrintf("%s — sending notification", label)
					go Notify(label, timeStr)
				}
				if cfg.Alarm.Azan {
					logPrintf("%s — playing azan", label)
					if err := Play(); err != nil {
						logPrintf("Warning: %v", err)
					}
				}
			}
			time.Sleep(30 * time.Second)
		}
	}
}
