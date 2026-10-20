# Assistant

An offline-first voice assistant for Linux and Windows, written in Go.

## Features

- **Voice and text input** — switch between modes at runtime.
- **Offline speech recognition** via [GigaSTT](https://github.com/sevagh/gigastt) (Rust) running locally.
- **Offline text-to-speech** via [Silero TTS](https://github.com/snakers4/silero-models) (Python) running locally.
- **Cross-platform** — Linux and Windows.
- **Configurable** — JSON config in the OS-standard config directory.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Microphone │────▶│   GigaSTT    │────▶│             │
└─────────────┘     │  (Rust, WS)  │     │             │
                    └──────────────┘     │             │
                                         │  Dispatcher │────▶ CLI
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

The Go binary is the "brain": it manages the lifecycle of GigaSTT and Silero, routes input from voice or CLI, and sends responses back to the CLI and TTS.

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

### Windows

- **MinGW-w64** (for CGO, required by `malgo`):
  - Install via [MSYS2](https://www.msys2.org/) or [w64devkit](https://github.com/skeeto/w64devkit).
  - Make sure `gcc` is in `PATH`.

## Build

### 1. Clone the repository

```sh
git clone https://github.com/sewaustav/assistant.git
cd assistant
```

### 2. Install dependencies

```sh
make install
```

This will:

- Check that Go, cargo, and Python are installed.
- Add `~/.cargo/bin` to `PATH` (in `~/.zshrc`).
- Install `gigastt` via `cargo install` (takes a few minutes on first run).
- Create a Python venv in `tts-server/.venv` and install Silero dependencies.

### 3. Build the binary

```sh
make build
```

The binary will be at `./bin/assistant`.

### 4. Run

```sh
make run
```

Or directly:

```sh
./bin/assistant
```

On first run, the app will:

1. Create a default config at:
   - **Linux:** `~/.config/assistant/config.json`
   - **Windows:** `%AppData%\assistant\config.json`
   - **macOS:** `~/Library/Application Support/assistant/config.json`
2. If `mode` is `voice`, start GigaSTT and Silero, then start listening.
3. If `mode` is `text`, skip servers and go straight to CLI.

## Configuration

Config file location (created automatically on first run):

| OS      | Path                                              |
|---------|---------------------------------------------------|
| Linux   | `~/.config/assistant/config.json`                 |
| Windows | `%AppData%\assistant\config.json`                 |
| macOS   | `~/Library/Application Support/assistant/config.json` |

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
    "server_url": "ws://127.0.0.1:9876/v1/ws"
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
  }
}
```

### Modes

- `voice` — microphone input via GigaSTT, responses via Silero TTS.
- `text` — CLI input, no servers started.

Switch modes at runtime with `/mode voice` or `/mode text`.

### CLI commands

| Command        | Description                  |
|----------------|------------------------------|
| `/mode voice`  | Switch to voice mode         |
| `/mode text`   | Switch to text mode          |
| `/quit`        | Exit the assistant           |

In voice mode, plain text input is ignored — only commands starting with `/` or `@` are processed.

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
│   ├── cli/                 # CLI input/output
│   ├── commands/            # keyword-based command parser (TODO)
│   ├── config/              # config loading + defaults
│   ├── dispatcher/          # mode switching, routing
│   ├── lifecycle/           # child process management
│   ├── llm/                 # LLM handler (TODO)
│   ├── logging/             # log file setup
│   └── speech_recognition/  # microphone + GigaSTT client
├── pkg/tts/                 # Silero TTS client + playback
├── tts-server/              # Python Silero server
├── Makefile
└── README.md
```

## Makefile targets

| Target        | Description                                  |
|---------------|----------------------------------------------|
| `make install`| Install Go deps, gigastt, and Python venv    |
| `make build`  | Build the binary to `./bin/assistant`        |
| `make run`    | Build and run                                |
| `make dev`    | Run via `go run` (no build)                  |
| `make test`   | Run tests                                    |
| `make lint`   | `go vet` + `gofmt`                           |
| `make clean`  | Remove `./bin`                               |

## Troubleshooting

### `gigastt: command not found`

Add cargo's bin directory to your `PATH`:

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

### `oto: context is already created`

This means the audio context was created twice. Ensure `tts.New()` is called once and the `Client` is reused. See `pkg/tts/tts.go`.

### Silero server fails to start

Check the log at `~/.cache/assistant/assistant.log` for the actual error. Common causes:

- Missing Python dependencies — run `make install-tts`.
- Port 9877 already in use — kill the old process.

## License

- **Go code:** MIT (see `LICENSE`)
- **Silero TTS models:** CC-NC-BY (non-commercial). Commercial use requires a separate license from Silero Team. See `NOTICE`.
- **GigaSTT:** see its repository for licensing.