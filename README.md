# when — Islamic prayer times CLI & alarm

> [!WARNING]
> **Disclaimer:** This project was "vibe coded" primarily for personal use that I decided to open-source. It is provided as-is, without any warranties.
>
> **USE AT YOUR OWN RISK**

Fetches daily Islamic prayer times from [Al-Manar](https://almanar.com.lb/salat/) and provides subcommands to display them, find the next prayer, or schedule audible azan alarms with desktop notifications.

## Installation

```bash
go install github.com/MAHcodes/when@latest
```

Or build from source:

```bash
git clone https://github.com/MAHcodes/when.git
cd when
go build -o when .
```

## Usage

```
when                          show usage
when help                     show usage
when version                  show version
when next [--countdown] [--json|--tsv]
                              show next upcoming prayer (or live countdown with --countdown)
when all [--json|--tsv]       show all prayer times
when refresh [--json|--tsv]   force refetch of prayer times and update the cache
when <prayer> [--json|--tsv]  show time for a specific prayer (case-insensitive)
when alarm [--daemon]         schedule azan alarms and desktop notifications
when systemd                  print systemd user service unit
when complete --zsh|--bash|--fish
                              print shell completion script
```

**Prayers:** `imsak`, `fajr`, `shorouq`, `dhuhr`, `asr`, `maghrib`, `isha`, `midnight`

### Examples

```bash
$ when all
Imsak — 03:46
Fajr — 03:57
Shorouq — 05:28
Dhuhr — 12:40
Asr — 16:25
Maghrib — 20:13
Isha — 21:17
Midnight — 23:55

$ when next --json
{"name":"Maghrib","remaining":"2h45m0s","time":"20:13"}

$ when fajr
Fajr — 03:57

$ when refresh
Imsak — 03:46
Fajr — 03:57
...

$ when alarm
Alarm started — Fajr, Dhuhr, Maghrib (azan: on, notify: on)
Press Ctrl+C to stop.

$ when alarm --daemon
Daemon started (PID 12345) — log: ~/.cache/when/alarm.log

$ when next --countdown
Maghrib — 20:13 (2h44m59s remaining)
```

### Machine-readable output

Use `--json` or `--tsv` with `all`, `refresh`, `next`, or any prayer name:

```bash
when all --json        # JSON array of all prayers
when all --tsv         # tab-separated values
when refresh --json    # refetch, update cache, and print all prayers as JSON
when next --json       # single prayer object with remaining duration
when fajr --json       # single prayer object
```

### Shell completions

Zsh:
```zsh
source <(when complete --zsh)
```

Bash:
```bash
source <(when complete --bash)
```

Fish:
```fish
when complete --fish | source
```

### Systemd user service

Run `when alarm` as a persistent user service:

```bash
when systemd > ~/.config/systemd/user/when.service
systemctl --user daemon-reload
systemctl --user enable --now when.service
```

## Configuration

Create `~/.config/when/config.json`:

```json
{
  "alarm": {
    "prayers": ["Fajr", "Dhuhr", "Maghrib"],
    "azan": true,
    "notify": true
  }
}
```

| Field        | Default                                  | Description                        |
|--------------|------------------------------------------|------------------------------------|
| `prayers`    | `["Fajr", "Dhuhr", "Maghrib"]`           | Which prayers trigger alarm/notify |
| `azan`       | `true`                                   | Play azan audio at prayer time     |
| `notify`     | `true`                                   | Send desktop notification          |
| `imsakOffset`| `0`                                      | Minutes before Fajr to trigger suhoor alarm |

Settings must live under `alarm`; keys written at the top level are accepted too, and unknown or invalid settings produce a warning on startup instead of being silently ignored.

Desktop notifications use `notify-send` (Linux) or `osascript` (macOS).

## Offline support

Prayer times are cached to `~/.cache/when/prayer-times.json`. If the network is unavailable, the cached data for the current day is used. The fetch retries 3 times with exponential backoff before falling back.

## How it works

Each invocation checks the cache first. If no cached data exists for today, it fetches the HTML page from Al-Manar, parses the prayer times from the `mnr-salat-times` list, translates the Arabic names to English, and caches the result. Subsequent commands on the same day are instant — no network request is made.

The alarm command (`when alarm`) stays running, checking the current time against the configured prayers. At the scheduled time it plays the embedded azan audio via `ffplay`, `paplay`, or `aplay`, and sends a desktop notification.

## License

MIT
