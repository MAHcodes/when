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
		// Fetch the prayer times once. We only refetch when the day rolls
		// over at midnight, not after every salat notification.
		prayers, err := prayer.Fetch()
		if err != nil {
			logPrintf("Warning: fetch failed: %v", err)
			if !waitOrStop(sigCh, 5*time.Minute) {
				logPrintf("Alarm stopped.")
				return nil
			}
			continue
		}

		allEntries, err := prayer.TodayEntries(prayers)
		if err != nil {
			logPrintf("Warning: no alarm times available, retrying...")
			if !waitOrStop(sigCh, time.Minute) {
				logPrintf("Alarm stopped.")
				return nil
			}
			continue
		}

		entries := buildEntries(allEntries, alarmSet, cfg.Alarm.ImsakOffset)
		if len(entries) == 0 {
			logPrintf("Warning: no alarm times available, retrying...")
			if !waitOrStop(sigCh, time.Minute) {
				logPrintf("Alarm stopped.")
				return nil
			}
			continue
		}

		now := time.Now()
		for _, e := range entries {
			if !e.time.After(now) {
				continue
			}

			logPrintf("Next alarm: %s at %s", e.label, e.time.Format("15:04"))
			if !waitOrStop(sigCh, time.Until(e.time)) {
				logPrintf("Alarm stopped.")
				return nil
			}

			fire(e, cfg.Alarm)
			waitOrStop(sigCh, 30*time.Second)
		}

		// Every alarm for today has fired. Hold the schedule until midnight,
		// then refresh the prayer times for the new day.
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.Local)
		logPrintf("All alarms done — refetching at midnight")
		if !waitOrStop(sigCh, time.Until(midnight)) {
			logPrintf("Alarm stopped.")
			return nil
		}
	}
}

type alarmEntry struct {
	label string
	time  time.Time
}

func buildEntries(all []prayer.TimeEntry, alarmSet map[string]bool, imsakOffset int) []alarmEntry {
	var entries []alarmEntry
	for _, e := range all {
		if !alarmSet[e.Prayer.Name] {
			continue
		}
		entries = append(entries, alarmEntry{e.Prayer.Name, e.Time})
		if e.Prayer.Name == "Fajr" && imsakOffset > 0 {
			suhoor := e.Time.Add(-time.Duration(imsakOffset) * time.Minute)
			entries = append(entries, alarmEntry{"Suhoor", suhoor})
		}
	}
	return entries
}

func fire(e alarmEntry, cfg config.AlarmConfig) {
	timeStr := e.time.Format("15:04")

	if e.label == "Suhoor" {
		logPrintf("%s — time for suhoor", timeStr)
		if cfg.Notify {
			logPrintf("%s — sending notification", e.label)
			go Notify("Suhoor", timeStr)
		}
		return
	}

	if cfg.Notify {
		logPrintf("%s — sending notification", e.label)
		go Notify(e.label, timeStr)
	}
	if cfg.Azan {
		logPrintf("%s — playing azan", e.label)
		if err := Play(); err != nil {
			logPrintf("Warning: %v", err)
		}
	}
}

// waitOrStop blocks for d, returning true when the duration elapses, or false
// if a stop signal is received first.
func waitOrStop(sigCh <-chan os.Signal, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	select {
	case <-sigCh:
		return false
	case <-time.After(d):
		return true
	}
}
