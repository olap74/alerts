# Alert

## Description

**Alert** is a program for monitoring alarms via API and automatically playing sound signals when the alarm state changes in a specified region. It supports event logging, multiple alarm types, repeated signal playback, and flexible configuration via a JSON file.

The program takes information about Alerts from [Ukrainealarm](https://api.ukrainealarm.com/)

## Main Features

- Periodically polls the alarm API.
- Determines alarm state (active/inactive), event time, region, and alarm type.
- Plays the corresponding mp3 file at the start, end, and repeat of an alarm.
- Logs events to a file and/or console.
- Stores alarm state in `state.json` for correct operation between restarts.
- Repeats the alarm signal at a specified interval if the alarm remains active.
- Does not play the repeat signal more than once per minute.

## Quick Start

```sh
go run main.go
```

## Installation

### Dependencies

- [github.com/faiface/beep](https://github.com/faiface/beep) — for mp3 playback.
- [github.com/faiface/beep/speaker](https://pkg.go.dev/github.com/faiface/beep/speaker)
- [github.com/faiface/beep/mp3](https://pkg.go.dev/github.com/faiface/beep/mp3)

Install Go dependencies:

```sh
go get github.com/faiface/beep
go get github.com/faiface/beep/mp3
go get github.com/faiface/beep/speaker
```

### System dependencies (for Ubuntu/Debian)

For correct sound playback, install the following packages:

```sh
sudo apt update
sudo apt install alsa-utils libasound2-doc libasound2-plugins libasound2-dev
```

### Build

For Windows `timetzdata` should be included to avoid of Timezone issues:

```
go build -tags timetzdata -o alerts .
```

For non-Windows systems the build can be done running:

```sh
go build -o alerts .
```

## Configuration file format (`config.json`)

```json
{
  "api_url": "https://api.ukrainealarm.com/api/v3/alerts/1313",
  "auth_header": "api_token",
  "audio_files": {
    "AIR": "sounds/start.mp3",
    "ARTILLERY": "sounds/start.mp3",
    "URBAN_FIGHTS": "sounds/start.mp3",
    "CHEMICAL": "sounds/start.mp3",
    "NUCLEAR": "sounds/start.mp3",
    "UNKNOWN": "sounds/start.mp3"
  },
  "alert_on_empty": "sounds/finish.mp3",
  "time_zone": "Europe/Warsaw",
  "log_to_file": true,
  "log_to_console": true,
  "log_file_path": "app.log",
  "log_level": 1,
  "repeat_audio_file": "sounds/continue.mp3",
  "enable_repeat_audio": true,
  "repeat_interval_min": 3,
  "request_interval_sec": 5
}
```

`api_token` should be changed to the real auth token that can be requested from [api.ukrainealarm.com/](https://api.ukrainealarm.com/)

### Options description

- **api_url**: URL for alarm API requests.
- **auth_header**: Authorization token for the API.
- **audio_files**: Mapping of alarm type to mp3 file to play at alarm start.
- **alert_on_empty**: mp3 file to play when the alarm ends.
- **time_zone**: Time zone for correct time display in logs.
- **log_to_file**: Log events to file (`true`/`false`).
- **log_to_console**: Log events to console (`true`/`false`).
- **log_file_path**: Path to the log file (default: `alert.log`).
- **log_level**: Log verbosity:
  - `0` — no output
  - `1` — only alarm state changes and sound playback events
  - `2` — also current alarm state on every API request
  - `3` — maximum details (all state changes, saves, etc.)
- **repeat_audio_file**: mp3 file for repeat signal during prolonged alarms.
- **enable_repeat_audio**: Enable repeat signal playback (`true`/`false`).
- **repeat_interval_min**: Interval (in minutes) between repeat signals.
- **request_interval_sec**: Interval (in seconds) between API requests.

## Notes

- Make sure all mp3 files exist at the specified paths.
- The program is cross-platform and works on Linux, Windows, and MacOS.
- For correct time handling, set the `time_zone` parameter properly.

## Releases

Download the latest binaries for your OS:

- [Windows (.exe)](https://github.com/olap74/alerts/releases/latest/download/alerts-windows-amd64.exe)
- [Linux (amd64)](https://github.com/olap74/alerts/releases/latest/download/alerts-linux-amd64)
- [MacOS (Apple Silicon)](https://github.com/olap74/alerts/releases/latest/download/alerts-darwin-aarch64)
- [MacOS (Intel)](https://github.com/olap74/alerts/releases/latest/download/alerts-darwin-x86)

---

