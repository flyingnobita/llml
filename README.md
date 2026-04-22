# LLM Launcher (`llml`)

[![Go](https://img.shields.io/github/go-mod/go-version/flyingnobita/llml)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

![LLM Launcher TUI screenshot](assets/llml-screenshot.png)

**LLM Launcher** (`llml`) is a TUI for people who already have models on disk and are
tired of reconstructing launch commands from shell history.

It scans your local filesystem for **GGUF** and **Hugging Face-style safetensors** models,
detects installed runtimes (**[llama.cpp](https://github.com/ggerganov/llama.cpp)**,
**[vLLM](https://github.com/vllm-project/vllm)**, and **[Ollama](https://ollama.com/)**),
and lets you save named parameter profiles per model — so the command that worked
last time is always one keystroke away.

Browse local models. Detect the right runtime. Launch with one key.

## ✨ Features

- **Model discovery** — auto-scans common paths for GGUF files and safetensors model
  directories; add extra roots via `LLML_MODEL_PATHS` and/or `config.toml`. Results are
  cached under **`{UserConfigDir}/llml/config.toml`** so the next launch can skip the
  filesystem walk when the cache is still valid.
- **Runtime detection** — finds installed `llama-server` and `vllm` binaries and maps
  installed `ollama` plus the configured Ollama host, then maps each model to its
  compatible runtime.
- **Named parameter profiles** — save multiple profiles per model (e.g. `fast-laptop`,
  `quality`, `api-8080`), each storing runtime args, env vars, port, and context
  settings. The active profile is always one key away.
- **One-keystroke launch** — select a model, select a profile, press `R`. The generated
  command is shown before execution and server output streams directly in the TUI.
- **Persistent status and alert history** — long-running work such as Ollama preloads stays
  visible in a persistent status line, while warnings and errors remain inspectable in a
  dedicated alert-history pane.
- **Ollama preload flow** — Ollama models are discovered via the Ollama API. Pressing
  `R` starts `ollama serve` if needed, then preloads the selected model into the
  shared Ollama service with `keep_alive: -1`.
- **Zero required setup** — common model directories and binary locations are checked
  automatically; configure only what differs from the defaults.

## 🚀 Quick start

### Runtime Requirements

- **Runtime engine (at least one)**: **llama.cpp** (`llama-server`) for GGUF models,
  **vLLM** (`vllm`) for safetensors models, and/or **Ollama** (`ollama`) for Ollama
  models are installed (see [Runtime Engines](#runtime-engines)).
- **Models** in default scan locations, or configure custom roots with `LLML_MODEL_PATHS` (see [Model Discovery](#model-discovery)).

### Install

Pick one path; you only need a single install method.

#### Go (`go install`)

Requires [Go 1.26+](go.mod). Ensure `$(go env GOPATH)/bin` is on your `PATH`.

```bash
go install github.com/flyingnobita/llml/cmd/llml@latest
```

#### Homebrew

```bash
brew tap flyingnobita/llml
brew install llml
```

Upgrade later with `brew upgrade llml`.

#### Pre-built binaries

For each [GitHub release](https://github.com/flyingnobita/llml/releases), archives are published for Linux and macOS (`tar.gz`) plus Windows (`zip`). Names follow GoReleaser’s pattern, for example `llml_1.2.3_Linux_x86_64.tar.gz`, `llml_1.2.3_Darwin_arm64.tar.gz`, or `llml_1.2.3_Windows_x86_64.zip` (adjust version and OS/arch to match your download). Extract the `llml` binary.

```bash
# Example: Linux x86_64 — use the archive name from the release you downloaded
tar -xzf llml_1.2.3_Linux_x86_64.tar.gz
chmod +x llml
```

Install on your `PATH` if you like (Linux/macOS/WSL):

```bash
sudo mv llml /usr/local/bin/llml
```

No-sudo option:

```bash
mkdir -p "$HOME/.local/bin"
mv llml "$HOME/.local/bin/llml"
```

(Ensure `~/.local/bin` is on your `PATH`.)

Verify the download against `llml_<version>_checksums.txt` on the release page if you rely on checksums (for example `llml_1.2.3_checksums.txt`).

### Build from source

#### Build Requirements

- **Go** [1.26+](go.mod)

```bash
git clone https://github.com/flyingnobita/llml.git
cd llml
go build -o llml ./cmd/llml
```

To install a development build from your clone, use `go install ./cmd/llml` from the repo root, or copy the `llml` binary onto your `PATH`.

### Start

```bash
llml
```

`llml` will automatically scan common locations for models and binaries. If your setup is non-standard (e.g., binaries not on `PATH` or models in custom folders), see the [Configuration](#configuration) section to point the app to the right directories. Select a model in the UI and press `R` to launch.

## ⌨️ Usage

| Key         | Action                                                                             |
| ----------- | ---------------------------------------------------------------------------------- |
| `hjkl/↑↓←→` | Move selection; horizontal scroll when the path column is wider than the terminal  |
| `r`         | Reload **`[runtime]`** from `config.toml` and re-detect binaries (no model rescan) |
| `S`         | Full model filesystem rescan; refresh cached **`[[models]]`** in `config.toml`     |
| `R`         | Run server (split view: table + log pane)                                          |
| `ctrl`+`R`  | Run server full-screen                                                             |
| `c`         | Edit runtime environment (paths, ports)                                            |
| `p`         | Edit parameter profiles for the selected model                                     |
| `m`         | Edit extra model search paths (saved in `config.toml`)                             |
| `,` / `.`   | Change sort column / reverse sort direction                                        |
| `enter`     | Copy the launch command for the selected row to the clipboard                      |
| `a`         | Toggle alert history pane                                                          |
| `t`         | Cycle theme (`dark` → `light` → `auto` → …)                                        |
| `?`         | Toggle the full shortcut help overlay                                              |
| `q`         | Quit                                                                               |

### Server output

**`R`** runs the server in a **split layout**: the model table stays in the upper half and server logs stream into a scrollable pane below. **tab** switches focus between the two panes; **esc**, **q**, or **ctrl+c** stops the server.

**ctrl+`R`** runs full-screen: the TUI is suspended and the server process is attached directly to your terminal. On Linux/macOS, after the server exits you are prompted to press Enter before the TUI redraws. On Windows there is no Enter prompt; you return when the server process exits.

For **Ollama** rows, `R` and `ctrl`+`R` do not start a dedicated per-model port.
Instead, `llml` ensures the shared Ollama daemon is running on the configured host
and preloads the selected model into memory with `keep_alive: -1`. The selected row
still matters, but the service endpoint remains the shared Ollama host.

### Status and alerts

`llml` separates active work from alert history:

- a persistent status line shows in-flight operations such as starting Ollama or loading a model
- `a` opens a bottom alert-history pane with timestamped `INFO`, `WARN`, and `ERROR` entries
- when the pane is closed, the footer shows an unread alert count

Minor confirmations such as copy-to-clipboard remain transient. Important failures and lifecycle
events remain inspectable in alert history until you dismiss or replace them by later work.

### Parameter profiles (`p`)

Each model path can have **multiple named profiles**. Each profile stores:

- **Environment variables** (`KEY=value` per line).
- **Extra arguments** (`--flag value` per line).

**`R`** / **ctrl+`R`** use the **active** profile (the highlighted row in the `p` profile list is prefixed with **`(active)`** in the name column). Changes persist automatically. **tab** cycles: profile list → env → extra args. On the profile list: **`a`** add profile, **`c`** clone (duplicate) the highlighted profile, **`d`** delete (not the last), **`r`** rename. **`esc`** closes the panel (and **`n`** cancels a delete confirmation).

Profiles are stored in `model-params.json` (see [Storage & Locations](#storage--locations)).

<a id="configuration"></a>

## ⚙️ Configuration

`llml` follows a strict precedence for settings:  
**Environment Variables** (highest) → **`config.toml`** → **Default/Auto-detected paths** (lowest).

### Storage & Locations

User data and settings are stored in a dedicated folder. Routine app upgrades **do not** delete these files.

| Platform    | Path                                                 |
| ----------- | ---------------------------------------------------- |
| Linux (XDG) | `$XDG_CONFIG_HOME/llml/` (usually `~/.config/llml/`) |
| macOS       | `~/Library/Application Support/llml/`                |
| Windows     | `%AppData%\llml\`                                    |

**Key Files:**

- **`config.toml`**: Stores global settings (ports, binary paths) and the model discovery cache.
- **`model-params.json`**: Stores your [named parameter profiles](#parameter-profiles-p) (args/env) for each model.
- **`backups/`**: Automatic timestamped snapshots created before the app overwrites configuration.

---

### Runtime Engines

Configure how `llml` finds and launches servers. You can edit these interactively in the UI (**`c`**).

| Feature            | Environment Variable | `config.toml` key (under `[runtime]`) | Default           |
| :----------------- | :------------------- | :------------------------------------ | :---------------- |
| **llama.cpp path** | `LLAMA_CPP_PATH`     | `default_llama_cpp_path`              | _(auto)_          |
| **llama.cpp port** | `LLAMA_SERVER_PORT`  | `default_llama_server_port`           | `8080`            |
| **vLLM path**      | `VLLM_PATH`          | `default_vllm_path`                   | _(auto)_          |
| **vLLM venv**      | `VLLM_VENV`          | `default_vllm_venv`                   | _(auto)_          |
| **vLLM port**      | `VLLM_SERVER_PORT`   | `default_vllm_server_port`            | `8000`            |
| **Ollama path**    | `OLLAMA_PATH`        | `default_ollama_path`                 | _(auto)_          |
| **Ollama host**    | `OLLAMA_HOST`        | `default_ollama_host`                 | `127.0.0.1:11434` |
| **TUI Theme**      | `LLML_THEME`         | -                                     | `auto`            |

**Detection Logic:**

1. Explicitly configured paths (Env/TOML).
2. Common system directories (e.g., `/usr/local/bin`, `/opt/homebrew/bin`, `~/.local/bin`).
3. Binary names available on your system `PATH`.
4. (llama.cpp only) Probing for an already-running server on the configured port.
5. (vLLM only) Common venv locations (e.g., `~/.venv-vllm-metal/bin` on macOS).

---

### Model Discovery

`llml` scans several common directories (e.g., `~/models`, `~/.cache/llama.cpp`, `~/.cache/lm-studio/models`) by default.

#### Custom Search Paths

Add extra directories to scan via the UI (**`m`**), `config.toml` (`discovery.extra_model_paths`), or environment variables:

| Environment Variable    | Role                                                                                                            |
| :---------------------- | :-------------------------------------------------------------------------------------------------------------- |
| `LLML_MODEL_PATHS`      | Comma-separated list of extra roots to scan. <br>Example: `export LLML_MODEL_PATHS="/data/models,/opt/weights"` |
| `HUGGINGFACE_HUB_CACHE` | Specific root for Hugging Face hub cache (overrides `HF_HOME/hub`).                                             |
| `HF_HOME`               | Root for HF home (cache resolves to `$HF_HOME/hub`).                                                            |

Example `config.toml` entry:

```toml
[discovery]
extra_model_paths = ["/data/models", "/opt/weights"]
```

#### Discovery Rules

The app recognizes the following model types:

- **GGUF**: Files ending in `.gguf`.
- **Safetensors**: Directories containing `config.json` and `*.safetensors` files (Hugging Face style).

---

### Data Integrity & Backups

To protect your settings and cache, `llml` maintains a history of your configuration:

- **Atomic Writes**: Files are written to a temporary location before being moved, preventing corruption.
- **Automatic Backups**: The newest **10** versions of each file are kept in the `backups/` directory.
- **Upgrade Snapshots**: When the `llml` version changes, a snapshot of both `config.toml` and `model-params.json` is created automatically so you can roll back if needed. The file **`.last-run-version`** in the same directory records the last run version for that behavior.

## 💻 Development

### Development Tooling Requirements

- **[mise](https://mise.jdx.dev/)** — manages Go, Node.js, and tasks. [Install mise](https://mise.jdx.dev/getting-started.html) first.
- **Go** [1.26+](go.mod) and **Node.js** (LTS) — both installed automatically by `mise install`.

### Set up tooling

Clone the repository and install tooling:

```bash
mise install   # installs Go, Node.js, pre-commit (pipx), and other tools from mise.toml
npm ci         # installs Prettier + markdownlint
pre-commit install   # optional: enable git pre-commit / pre-push hooks (see .pre-commit-config.yaml)
```

### Common Tasks

```bash
mise run run      # go run ./cmd/llml
mise run build    # build to bin/llml
mise run format   # auto-fix: gofmt + prettier + markdownlint
mise run lint     # check only: gofmt + vet + prettier + markdownlint
mise run test     # go test -race ./...
mise run check    # lint + test (run before opening a PR)
```

### Layout

- `cmd/llml` — entrypoint.
- `internal/config` — `config.toml` read/write and cache helpers.
- `internal/tui` — Bubble Tea UI.
- `internal/models` — discovery, metadata, runtime detection.
- `scripts/` — `gofmt-check.sh`, `precommit-docs-fix.sh`.

Contributions are welcome. Please run `mise run check` before opening a pull request.

## ❤️ Support This Project

You can support this project in a few simple ways:

- ⭐ [Star the repo](https://github.com/flyingnobita/llml)
- 🐛 [Report bugs](https://github.com/flyingnobita/llml/issues)
- 💡 [Suggest features](https://github.com/flyingnobita/llml/issues)
- 📝 [Contribute code](https://github.com/flyingnobita/llml/pulls)

## 📄 License

[MIT](LICENSE) © Flying Nobita
