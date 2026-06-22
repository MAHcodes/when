package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MAHcodes/when/alarm"
	"github.com/MAHcodes/when/prayer"
)

const version = "1.0.0"

var helpText = `when — Islamic prayer times CLI & alarm

Usage:
  when                    show usage
  when help               show usage
  when version            show version
  when next [--countdown] [--json|--tsv]
                          show next upcoming prayer (or live countdown with --countdown)
  when all [--json|--tsv] show all prayer times
  when <prayer> [--json|--tsv]
                          show time for a specific prayer (case-insensitive)
  when alarm [--daemon]   schedule azan alarms and desktop notifications
  when systemd            print systemd user service unit
  when complete --zsh|--bash|--fish
                          print shell completion script

Prayers: imsak, fajr, shorouq, dhuhr, asr, maghrib, isha, midnight
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(helpText)
		return
	}

	cmd := os.Args[1]
	rest := os.Args[2:]

	format := ""
	daemon := false
	countdown := false
	for _, a := range rest {
		switch a {
		case "--json":
			format = "json"
		case "--tsv":
			format = "tsv"
		case "--daemon":
			daemon = true
		case "--countdown":
			countdown = true
		}
	}

	switch cmd {
	case "help", "--help", "-h":
		fmt.Print(helpText)
	case "version", "--version", "-v":
		fmt.Println("when version", version)
	case "next":
		runNext(format, countdown)
	case "all":
		runAll(format)
	case "alarm":
		runAlarm(daemon)
	case "systemd":
		printSystemd()
	case "complete":
		printCompletion(rest)
	default:
		runPrayer(cmd, format)
	}
}

func runNext(format string, countdown bool) {
	if countdown {
		runCountdownLoop()
		return
	}

	prayers, err := prayer.Fetch()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching prayer times: %v\n", err)
		os.Exit(1)
	}

	p, dur, err := prayer.Next(prayers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error computing next prayer: %v\n", err)
		os.Exit(1)
	}

	switch format {
	case "json":
		remaining := dur.Round(time.Second).String()
		fmt.Println(prayer.NextJSON(p.Name, p.Time, remaining))
	case "tsv":
		fmt.Printf("%s\t%s\n", p.Name, p.Time)
	default:
		remaining := dur.Round(time.Second)
		fmt.Printf("%s — %s (%s remaining)\n", p.Name, p.Time, remaining)
	}
}

func runAll(format string) {
	prayers, err := prayer.Fetch()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching prayer times: %v\n", err)
		os.Exit(1)
	}

	switch format {
	case "json":
		s, err := prayer.PrayersJSON(prayers)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(s)
	case "tsv":
		fmt.Print(prayer.PrayersTSV(prayers))
	default:
		for _, p := range prayers {
			fmt.Printf("%s — %s\n", p.Name, p.Time)
		}
	}
}

func runPrayer(name string, format string) {
	prayers, err := prayer.Fetch()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching prayer times: %v\n", err)
		os.Exit(1)
	}

	p, ok := prayer.Find(prayers, name)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown prayer: %s\n", name)
		os.Exit(1)
	}

	switch format {
	case "json":
		fmt.Println(prayer.SingleJSON(p.Name, p.Time))
	case "tsv":
		fmt.Printf("%s\t%s\n", p.Name, p.Time)
	default:
		fmt.Printf("%s — %s\n", p.Name, p.Time)
	}
}

func runCountdownLoop() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		prayers, err := prayer.Fetch()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching prayer times: %v\n", err)
			time.Sleep(5 * time.Minute)
			continue
		}

		for {
			p, dur, err := prayer.Next(prayers)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error computing next prayer: %v\n", err)
				os.Exit(1)
			}

			remaining := dur.Round(time.Second)
			if remaining <= 0 {
				fmt.Printf("\r%s!\033[K\n", p.Name)
				alarm.Play()
				break
			}

			fmt.Printf("\r%s — %s (%s remaining)\033[K", p.Name, p.Time, remaining)

			select {
			case <-sigCh:
				fmt.Println("\nCountdown stopped.")
				return
			case <-time.After(time.Second):
			}
		}
	}
}

func runAlarm(daemon bool) {
	if daemon {
		if err := alarm.Daemonize(); err != nil {
			fmt.Fprintf(os.Stderr, "Error daemonizing: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := alarm.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running alarm: %v\n", err)
		os.Exit(1)
	}
}

func printSystemd() {
	exe, err := os.Executable()
	if err != nil {
		exe = "when"
	}
	fmt.Printf(`[Unit]
Description=when — Islamic prayer times alarm
After=network.target

[Service]
ExecStart=%s alarm
Restart=on-failure
RestartSec=60

[Install]
WantedBy=default.target
`, exe)
}

func printCompletion(flags []string) {
	shell := "zsh"
	for _, f := range flags {
		switch f {
		case "--bash":
			shell = "bash"
		case "--fish":
			shell = "fish"
		case "--zsh":
			shell = "zsh"
		}
	}

	prayers := "imsak fajr shorouq dhuhr asr maghrib isha midnight"
	cmds := "help version next all alarm systemd complete " + prayers

	switch shell {
	case "bash":
		fmt.Printf(`# bash completion for when
_when() {
  local cur=$2
  if [ "$cur" = "" ]; then
    COMPREPLY=( $(compgen -W "%s" -- "$cur") )
  fi
}
complete -F _when when
`, cmds)
	case "fish":
		fmt.Printf(`# fish completion for when
complete -c when -f -a "help version next all alarm systemd complete %s"
`, prayers)
	default:
		fmt.Printf(`#compdef when

_arguments \
  '1: :(%s)'
`, cmds)
	}
}
