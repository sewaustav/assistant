# Assistant

An offline-first voice assistant for **Linux** and **Windows**, written in Go.

Speak or type — the assistant recognizes speech locally with
[GigaSTT](https://github.com/sevagh/gigastt), answers through
[Silero TTS](https://github.com/snakers4/silero-models), and can control the
desktop: open apps, copy/paste, undo, change volume, take screenshots and more.

## Features

- **Voice and text input** — switch between modes at runtime.
- **Offline speech recognition** via GigaSTT (Rust, local WebSocket server).
- **Offline text-to-speech** via Silero TTS (Python, local HTTP server).
- **Cross-platform** — Linux, Windows, and basic macOS support.
- **Built-in desktop commands**:
  - clipboard: select all, copy, cut, paste, delete, undo, redo;
  - windows/tabs: close window, new tab, reload page, save;
  - system: volume up/down/mute, play/pause, next/prev track, lock screen,
    screenshot;
  - apps: open any installed application by name;
  - web: open browser, search the web, open popular sites;
  - input: type dictated text into the active field;
  - help: list available commands by voice.
- **App discovery** — Linux `.desktop` files, Windows Start Menu shortcuts,
  macOS `/Applications`, plus a built-in list of common system apps.
- **Configurable** — JSON config in the OS-standard config directory.
- **Optional LLM** — OpenAI-compatible tool calling, with a local fallback.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Microphone │────▶│   GigaSTT    │────▶│             │
└─────────────┘     │  (Rust, WS)  │     │             │
                    └──────────────┘     │  Dispatcher │────▶ CLI
┌─────────────┐                          │    (Go)     │
│  CLI input  │─────────────────────────▶│             │────▶ TTS
└─────────────┘                          └─────────────┘
                                                │
                                                ▼
                                         ┌─────────────┐
                                         │  Silero TTS │
                                         │(Python,HTTP)│
                                         └─────────────┘
```

The Go binary is the "brain": it manages the lifecycle of GigaSTT and Silero,
routes input from voice or CLI, executes desktop actions, and sends responses
back to the CLI and TTS.

## Requirements

### Common

- **Go 1.22+** — [download](https://go.dev/dl/)
- **Rust / cargo** — [install](https://rustup.rs/)
- **Python 3.10+** with `venv`
- **Git**

### Linux

- **ALSA development headers** (for microphone capture):
  ```sh
  # Arch
  sudo pacman -S alsa-lib
  # Debian/Ubuntu
  sudo apt install libasound2-dev
  ```
- **eSpeak NG** (required by GigaSTT for phonemization):
  ```sh
  # Arch
  sudo pacman -S espeak-ng
  # Debian/Ubuntu
  sudo apt install espeak-ng
  ```
- **Desktop control tools** (at least one per task):
  ```sh
  # Wayland: keyboard input
  sudo pacman -S wtype        # or: sudo apt install wtype
  # X11: keyboard input
  sudo pacman -S xdotool      # or: sudo apt install xdotool
  # Screenshots (any one)
  sudo pacman -S grim         # Wayland
  sudo pacman -S spectacle
  sudo pacman -S scrot
  # Clipboard fallback for "type text"
  sudo pacman -S wl-clipboard # or xclip / xsel
  ```
  `wtype`/`xdotool` are auto-detected. Without them, hotkey commands return a
  clear error.

### Windows

- **Go 1.22+** — the Windows build uses native `winmm` audio capture and does
  **not** require CGO or MinGW.
- **PowerShell 5+** (built into Windows) for screenshots and text input.
- GigaSTT must be installed with `cargo install gigastt`. It installs to
  `%USERPROFILE%\.cargo\bin\gigastt.exe`.

## Build

### Linux / macOS

```sh
git clone https://github.com/sewaustav/assistant.git
cd assistant
make install   # Go deps, gigastt, Python venv
make build
make run
```

The binary is at `./bin/assistant`.

### Windows (PowerShell)

```powershell
git clone https://github.com/sewaustav/assistant.git
cd assistant
powershell -ExecutionPolicy Bypass -File .\scripts\install-windows.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\run-windows.ps1
```

The binary is at `.\bin\assistant.exe`.

> The Windows build uses the native `winmm` API for microphone capture, so it
> works with `CGO_ENABLED=0` and does not need a C compiler. Linux and macOS
> still use `malgo` (CGO) for audio capture.

### Cross-compilation

```sh
make build-linux           # Linux amd64 (needs CGO/gcc)
make build-windows         # Windows amd64, no CGO/MinGW needed
make build-darwin          # macOS arm64, build on macOS or with osxcross
make build-all             # Linux + Windows
```

Windows can be cross-compiled from any OS with a standard Go toolchain:

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o bin/assistant.exe ./cmd/assistant_app
```

## Configuration

Config file location (created automatically on first run):

| OS      | Path                                              |
|---------|---------------------------------------------------|
| Linux   | `~/.config/assistant/config.json`                 |
| Windows | `%AppData%\assistant\config.json`                 |
| macOS   | `~/Library/Application Support/assistant/config.json` |

The command list lives next to it in `commands.json`. New built-in commands are
merged into an existing file automatically on startup; your own edits and
commands are preserved.

Example config:

```json
{
  "mode": "voice",
  "verbose": false,
  "tts": {
    "enabled": true,
    "server_url": "http://127.0.0.1:9877",
    "speaker": "xenia",
    "sample_rate": 48000,
    "echo_tail_ms": 300
  },
  "asr": {
    "enabled": true,
    "server_url": "ws://127.0.0.1:9876/v1/ws",
    "sample_rate": 48000,
    "vad": false,
    "model_dir": "",
    "extra_args": []
  },
  "llm": {
    "enabled": false,
    "provider": "openai",
    "base_url": "https://api.openai.com/v1",
    "model": "gpt-4o-mini",
    "timeout": 30,
    "api_key_env": "ASSISTANT_LLM_API_KEY"
  },
  "wake_word": {
    "enabled": false,
    "model_path": "",
    "threshold": 0.5,
    "cooldown_sec": 2
  },
  "paths": {
    "gigastt": "",
    "python": "",
    "tts_dir": ""
  },
  "search_url": "https://yandex.ru/search/?text={query}",
  "apps": {}
}
```

### Modes

- `voice` — microphone input via GigaSTT, responses via Silero TTS.
- `text` — CLI input, servers are not started.

Switch modes at runtime with `/mode voice` or `/mode text`.

If voice mode cannot start (GigaSTT missing, microphone error, ASR disabled),
the assistant does **not** crash: it prints the reason and switches to text mode.

### CLI commands

| Command        | Description                  |
|----------------|------------------------------|
| `/mode voice`  | Switch to voice mode         |
| `/mode text`   | Switch to text mode          |
| `/apps`        | List discovered applications |
| `/help`        | CLI help                     |
| `/version`     | Show version and platform    |
| `/quit`        | Exit the assistant           |

In voice mode, plain text input is ignored — only `/` and `@` commands are
processed. In text mode you can type normal phrases and the same command
matcher runs.

### Voice commands

Say any of these (Russian keywords are configured in `commands.json`):

| What                | Examples                                      |
|---------------------|-----------------------------------------------|
| Time / date         | «время», «который час», «какая сегодня дата»  |
| Open an app         | «открой телеграм», «запусти калькулятор»      |
| Browser / web       | «открой браузер», «открой ютуб», «карты», «погода», «загугли …» |
| Clipboard           | «выдели всё», «скопируй», «вставь», «вырежи»  |
| Undo / redo         | «отмени», «повтори действие»                  |
| Window / tabs       | «закрой окно», «новая вкладка», «обнови»      |
| Volume / media      | «громче», «тише», «выключи звук», «пауза»     |
| Lock / screenshot   | «заблокируй экран», «скриншот»                |
| Type text           | «напиши привет мир»                           |
| Help                | «помощь», «что ты умеешь»                     |

### Opening applications

The assistant discovers applications automatically:

- **Linux:** `/usr/share/applications`, `~/.local/share/applications`,
  Flatpak and Snap menus.
- **Windows:** Start Menu shortcuts (`.lnk`, `.url`).
- **macOS:** `/Applications`, `/System/Applications`, `~/Applications`.

There is also a built-in list: terminal, files, calculator, text editor,
settings, task manager, browser, music, camera, clock, photos.

If the requested name is a well-known site (YouTube, VK, Telegram Web,
GitHub, Yandex Maps, weather, mail), it opens in the browser even when no
desktop application is installed.

To add your own app, put it in `config.json`:

```json
"apps": {
  "telegram": {
    "name": "Telegram",
    "aliases": ["телеграм", "тг"],
    "linux": "telegram-desktop",
    "windows": "C:\\Users\\me\\AppData\\Roaming\\Telegram Desktop\\Telegram.exe",
    "darwin": "open -a Telegram"
  }
}
```

### LLM (optional)

Set `llm.enabled` to `true`, provide an OpenAI-compatible `base_url`, `model`,
and the API key in the environment variable named by `api_key_env`:

```sh
export ASSISTANT_LLM_API_KEY="sk-..."
```

The handler sends the tool schema, executes tool calls, and falls back to the
local keyword matcher if the LLM request fails. For local servers (Ollama,
LM Studio, vLLM) no API key is required when `base_url` points to localhost.

## Logs

Technical logs are written to:

| OS      | Path                                          |
|---------|-----------------------------------------------|
| Linux   | `~/.cache/assistant/assistant.log`            |
| Windows | `%LocalAppData%\assistant\assistant.log`      |
| macOS   | `~/Library/Caches/assistant/assistant.log`    |

The CLI only shows user-facing messages (`[you]`, `[assistant]`, `[info]`).

## Project structure

```
assistant/
├── cmd/assistant_app/       # main entry point
├── internal/
│   ├── apps/                # app discovery and launching
│   ├── cli/                 # CLI input/output
│   ├── config/              # config loading + defaults
│   ├── dispatcher/          # mode switching, routing
│   ├── lifecycle/           # child process management
│   ├── llm/                 # OpenAI-compatible tool calling
│   ├── local/               # keyword command matcher
│   ├── logging/             # log file setup
│   ├── paths/               # cross-platform path resolution
│   ├── speech_recognition/  # microphone + GigaSTT client
│   └── tools/               # desktop actions (hotkeys, volume, apps...)
│       └── platform/        # Linux / Windows / macOS backends
├── pkg/tts/                 # Silero TTS client + playback
├── scripts/                 # Windows PowerShell install/build/run
├── tts-server/              # Python Silero server
├── Makefile
└── README.md
```

## Makefile targets

| Target          | Description                                      |
|-----------------|--------------------------------------------------|
| `make install`  | Install Go deps, gigastt, and Python venv        |
| `make build`    | Build the binary for the host OS                 |
| `make build-windows` | Cross-compile `assistant.exe` (no CGO)      |
| `make run`      | Build and run                                    |
| `make dev`      | Run via `go run` (no build)                      |
| `make test`     | Run tests                                        |
| `make lint`     | `go vet` + `gofmt`                               |
| `make clean`    | Remove `./bin`                                   |

## Troubleshooting

### Windows: recognition does not start

1. Check that `gigastt` is installed:
   ```powershell
   gigastt --version
   ```
   If not, run `cargo install gigastt` and reopen the terminal so `PATH` picks up
   `%USERPROFILE%\.cargo\bin`.
2. Check the log at `%LocalAppData%\assistant\assistant.log`.
3. Make sure the microphone is allowed in **Settings → Privacy → Microphone**.
4. If the app prints that GigaSTT is missing, set the path explicitly:
   ```json
   "paths": { "gigastt": "C:\\Users\\me\\.cargo\\bin\\gigastt.exe" }
   ```
5. The assistant tries 48 kHz, 44.1 kHz, 32 kHz and 16 kHz capture and tells
   GigaSTT the actual rate. This fixes most Windows audio-device mismatches.
6. Windows microphone capture uses the built-in `winmm` API, so a C compiler
   and MinGW are not required. If `waveInOpen` fails, the log names the exact
   `MMRESULT` code and the app falls back to text mode.

### `gigastt: command not found`

Add cargo's bin directory to `PATH`:

```sh
export PATH="$HOME/.cargo/bin:$PATH"
```

Or re-run `make install` — it adds this to `~/.zshrc` automatically.

### `libespeak-ng.so.1: cannot open shared object file`

Install eSpeak NG:

```sh
# Arch
sudo pacman -S espeak-ng
# Debian/Ubuntu
sudo apt install espeak-ng
```

### Hotkeys do nothing on Linux

Install a keyboard tool:

- Wayland: `wtype`
- X11: `xdotool`

`SendHotkey` auto-detects the session type. On Wayland with only `xdotool`,
global hotkeys only work in XWayland windows.

### Screenshot command fails

Install one of: `grim`, `gnome-screenshot`, `spectacle`, `scrot`, or
ImageMagick (`import`). On Windows, PowerShell's `System.Drawing` is used.

### Silero server fails to start

Check the log for the actual error. Common causes:

- Missing Python dependencies — run `make install-tts` (or the Windows
  PowerShell install script).
- Port 9877 already in use — kill the old process.
- The app could not find `tts-server` — set `paths.tts_dir` in `config.json`.

### `oto: context is already created`

The audio context is created lazily, only once, on the first TTS response.
If you see this, make sure `pkg/tts.Client` is reused, not recreated per
response.

## License

- **Go code:** MIT (see `LICENSE`)
- **Silero TTS models:** CC-NC-BY (non-commercial). Commercial use requires a
  separate license from Silero Team. See `NOTICE`.
- **GigaSTT:** see its repository for licensing.
