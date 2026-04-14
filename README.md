# LLM Launcher

[![Go](https://img.shields.io/github/go-mod/go-version/flyingnobita/llml)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**LLM Launcher** (`llml`) is a terminal UI for discovering local **GGUF** and **Hugging Face-style safetensors** models and launching **[llama.cpp](https://github.com/ggerganov/llama.cpp)** (`llama-server`) or **[vLLM](https://github.com/vllm-project/vllm)** (`vllm serve`) for the selected row.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), and a small fork of the Bubbles table (per-cell selection).

## Table of contents

- [Features](#features)
- [Requirements](#requirements)
- [Install](#install)
- [Quick start](#quick-start)
- [Usage](#usage)
- [Configuration](#configuration)
- [How it finds models](#how-it-finds-models)
- [Runtime detection](#runtime-detection)
- [Development](#development)
- [License](#license)

## Features

- **One table** for GGUF files and safetensors checkpoints: name, path, size, last modified, and parameter summary (GGUF metadata or `config.json` for vLLM).
- **Rescan** with `r`; **launch** the server for the selected row with **`R`** (shift+r): `llama-server` for GGUF, `vllm serve` for checkpoint directories.
- **Runtime editor** (`c`): edit `LLAMA_CPP_PATH`, `VLLM_PATH`, `VLLM_VENV`, `LLAMA_SERVER_PORT`, and `VLLM_SERVER_PORT` in the TUI (tab / shift+tab between fields).
- **Parameter profiles** (`p`): per-model named profiles of extra environment variables and CLI arguments; persisted under your OS config directory (see below).
- **Sensible defaults** for scan roots and binary resolution, overridable via environment variables.

## Requirements

- **Go** [1.26+](go.mod) only if you build from source or work on the project.
- **llama.cpp** binaries (`llama-server`, and optionally `llama-cli`) for GGUF rows, and/or **vLLM** (`vllm`) for safetensors rows, installed where the app can find them (see [Runtime detection](#runtime-detection)).
- **Node.js** (LTS) only if you run project checks (`npm ci` + Prettier / markdownlint via `mise run check`).

## Install

### Pre-built binaries (no Go)

For each [GitHub release](https://github.com/flyingnobita/llml/releases), archives are published for Linux and macOS (`tar.gz`) and Windows (`zip`) on **amd64** and **arm64** (Windows is amd64 only). Download the archive for your OS and CPU, extract the `llml` binary (or `llml.exe` on Windows), and place it on your `PATH`.

Verify the download against `llml_<version>_checksums.txt` on the release page if you rely on checksums.

### From source

```bash
git clone https://github.com/flyingnobita/llml.git
cd llml
go build -o llml ./cmd/llml
```

Install on your `PATH` if you like:

```bash
go install github.com/flyingnobita/llml/cmd/llml@latest
```

(Ensure `$(go env GOPATH)/bin` is on your `PATH`.)

### With mise (optional)

If you use [mise](https://mise.jdx.dev/), the repo includes tasks for run, build, and full checks:

```bash
mise install
mise run build    # binary: bin/llml
```

## Quick start

```bash
./llml
# or: mise run run
```

Place models under default scan locations (see [How it finds models](#how-it-finds-models)) or set `LLM_LAUNCH_LLAMACPP_PATHS`. Point `LLAMA_CPP_PATH` / `VLLM_PATH` at your install dirs if binaries are not on `PATH`.

## Usage

| Key                | Action                                                       |
| ------------------ | ------------------------------------------------------------ |
| `↑` `↓` or `j` `k` | Move selection                                               |
| `r`                | Rescan filesystem                                            |
| **`R`**            | Run server for selected row (`llama-server` or `vllm serve`) |
| `c`                | Edit runtime environment (paths, ports)                      |
| `p`                | Edit parameter profiles for the selected model               |
| `q` or `Ctrl+C`    | Quit                                                         |

### Server output

By default, **`R`** uses Bubble Tea’s [`tea.ExecProcess`](https://github.com/charmbracelet/bubbletea): the alternate screen is released and the server process is attached to your terminal so logs print normally until exit. On **Linux/macOS**, a small `sh` wrapper echoes the exact command (line starting with `+`), runs the server, then prompts **Press Enter to return to LLM Launcher…** before restoring the TUI. On **Windows**, the server runs without that echo/pause.

You can also run the printed command manually in another terminal, or redirect output with your shell.

### Parameter profiles (`p`)

Each model path can have **multiple named profiles**. Each profile stores:

- **Environment variables** (`KEY=value` per line).
- **Extra arguments** appended after `--port` (for vLLM, flags and values are separate argv tokens; the UI may show `--flag value` on one line).

**`R`** uses the **active** profile (the one highlighted when you save with `s` in the `p` panel). **tab** cycles: profile list → env → extra args. In the list: **`n`** new profile, **`d`** delete (not the last), **Enter** rename.

Storage is a single JSON file (not environment variables):

| Platform    | Typical path                                                                    |
| ----------- | ------------------------------------------------------------------------------- |
| Linux (XDG) | `$XDG_CONFIG_HOME/llml/model-params.json` or `~/.config/llml/model-params.json` |
| macOS       | `~/Library/Application Support/llml/model-params.json`                          |
| Windows     | `%AppData%\llml\model-params.json`                                              |

## Configuration

There is **no** runtime `config.toml` and **no** automatic `.env` file. Behavior is driven by **environment variables** and the **parameter profiles** file above.

### Discovery

Default roots include `~/models`, `~/.cache/llama.cpp`, Hugging Face hub cache paths, and `~/.cache/lm-studio/models` (only existing directories are used).

Add extra roots (comma-separated):

```bash
export LLM_LAUNCH_LLAMACPP_PATHS="/data/models,/opt/weights"
```

`HUGGINGFACE_HUB_CACHE` / `HF_HOME` influence Hugging Face cache layout as usual.

### Ports and paths (runtime)

| Variable            | Default   | Role                                                                       |
| ------------------- | --------- | -------------------------------------------------------------------------- |
| `LLAMA_CPP_PATH`    | _(unset)_ | Directory containing `llama-cli` / `llama-server` (checked before `PATH`)  |
| `VLLM_PATH`         | _(unset)_ | Directory where `vllm` or `.venv/bin/vllm` may live                        |
| `VLLM_VENV`         | _(unset)_ | Python venv root; on Unix, **`R`** may `source bin/activate` before `vllm` |
| `LLAMA_SERVER_PORT` | `8080`    | Port for `llama-server` and `/health` probe                                |
| `VLLM_SERVER_PORT`  | `8000`    | Port for `vllm serve`                                                      |

Set these in your shell, or under `[env]` in `mise.toml` for local development.

## How it finds models

- **GGUF**: files ending in `.gguf` under the scan roots.
- **Safetensors (vLLM)**: a directory containing **`config.json`** and at least one **`*.safetensors`** file (Hugging Face-style checkpoint).

The table shows a decoded repo id from `models--*` hub folders when possible; otherwise folder names and paths are shown with `~/` shortened and truncation when the terminal is narrow.

## Runtime detection

On launch and on **`r`**, the app resolves **llama.cpp** and **vLLM** binaries, then scans for models.

### llama.cpp (`llama-cli` / `llama-server`)

1. **`LLAMA_CPP_PATH`** if set: `{LLAMA_CPP_PATH}/<binary>` must exist.
2. Common locations: `/usr/local/bin`, `/opt/homebrew/bin`, `/opt/llama.cpp/build/bin`, `~/.local/bin`.
3. **`PATH`** via `exec.LookPath`.

If both binaries are still missing, the app may probe a **running** llama.cpp server: HTTP `GET` `http://127.0.0.1:<LLAMA_SERVER_PORT>/health` (2s timeout, success = HTTP 200).

### vLLM (`vllm`)

1. **`VLLM_PATH`**: `{VLLM_PATH}/vllm` or `{VLLM_PATH}/.venv/bin/vllm`.
2. **`VLLM_VENV`**: `{VLLM_VENV}/bin/vllm` if present.
3. Common directories as above, then **`PATH`**.

On **Linux/macOS**, if vLLM lives in a venv, **`R`** may source `activate` before `vllm serve` (next to the resolved binary, or via `VLLM_VENV` / `.venv` heuristics). On **Windows**, use an activated shell or put `vllm` on `PATH`.

## Development

Clone the repository and install tooling:

```bash
mise install          # Go + pre-commit (optional)
npm ci                # Prettier + markdownlint
go mod download
```

Common tasks:

```bash
mise run run       # go run ./cmd/llml
mise run build     # bin/llml
mise run check     # fmt + vet + prettier + markdownlint + tests (race)
```

Layout:

- `cmd/llml` — entrypoint.
- `internal/tui` — Bubble Tea UI.
- `internal/tui/btable` — vendored table fork (per-cell selected styling).
- `internal/llamacpp` — discovery, metadata, runtime detection.

Contributions are welcome. Please run `mise run check` (or equivalent) before opening a pull request.

## License

[MIT](LICENSE)
