# llm-launch

Terminal UI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss), and
[Bubbles](https://github.com/charmbracelet/bubbles) (table + key bindings). Scaffolded from
[flyingnobita/project-template](https://github.com/flyingnobita/project-template).

## What it does (v1)

Scans common directories for:

- **GGUF** files (llama.cpp / `llama-server`), and
- **Hugging Face-style safetensors** checkpoints: a directory containing **`config.json`** and at least one **`*.safetensors`** file (vLLM / `vllm serve`).

All matches appear in one table:

| Column        | Meaning                                                                                                                                                                                                      |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Name          | GGUF file basename; for Hub safetensors dirs, decoded repo id from the `models--*` folder (e.g. `google/gemma-…`), else the checkpoint folder name                                                           |
| Path          | Model folder: Hugging Face hub cache stops at `models--*` (no `snapshots/<hash>/`); GGUF rows use the parent dir of the file; safetensors rows use the checkpoint dir; `~/` shortened; truncated when narrow |
| Size          | GGUF file size, or sum of `*.safetensors` sizes in that directory                                                                                                                                            |
| Last modified | File `mtime` (GGUF) or latest `mtime` among weight shards (safetensors)                                                                                                                                      |
| Parameters    | GGUF metadata (architecture, context length when present), or `config.json` summary for vLLM rows (`model_type`, first architecture)                                                                         |

Press `r` to rescan, **`R`** (shift+r) to run a server for the selected row: **`llama-server -m …`** for GGUF, or **`vllm serve …`** for safetensors checkpoints. **`c`** opens the **runtime environment** editor (same variables as the bottom panel: `LLAMA_CPP_PATH`, `VLLM_PATH`, `VLLM_VENV`, `LLAMA_SERVER_PORT`, `VLLM_SERVER_PORT`), prefilled and editable; **tab** / **shift+tab** moves between fields. **`p`** opens **parameter profiles** for the selected model (extra env vars and CLI args; see below). `q` or `Ctrl+C` quits. Use arrow keys or `j` / `k` to move in the table.

### Seeing server output (`llama-server` / `vllm`)

**Default (`R` in the app):** the TUI uses Bubble Tea’s [`tea.ExecProcess`](https://github.com/charmbracelet/bubbletea): it **releases the alternate screen** and attaches the server process **to your terminal**, so **logs print normally** until the process exits. On **Linux/macOS**, a **`sh` script** first **echoes the exact command** (line starting with **`+`**, shell-quoted paths), then runs the server, then prints **“Press Enter to return to llm-launch…”** and waits for **Enter** before the alternate screen comes back. On **Windows**, the server runs directly without that echo/pause (use scrollback or another window if needed).

**Alternatives:**

1. **Another terminal** — run the same command the app would use: `llama-server -m <path/to/model.gguf> --port <port>` (from `LLAMA_SERVER_PORT`, default **8080**) or `vllm serve <path/to/model/dir> --port <port>` (from `VLLM_SERVER_PORT`, default **8000**, matching vLLM’s usual listen port).
2. **Save logs** — shell redirect or `tee`.
3. **In-TUI log pane** — not implemented here (more moving parts than `ExecProcess`).

### Parameter profiles (`p`)

Each model path can have **multiple named parameter profiles**: each profile is one set of **environment variables** (`KEY=value` per line) and **extra arguments** passed after `--port`. For vLLM, each flag and its value are **two separate argv tokens** (e.g. `--max-model-len` and `4096`); the TUI usually shows them **on one line** (`--max-model-len 4096`). You can also use **one row per token**. A single line **`--flag value`** is split on the first space when saved. The echoed **`+`** command uses **minimal quoting** (e.g. `--max-model-len 4096` rather than two quoted chunks) while still passing the same argv to the shell.

- **`R`** uses the **active** profile (the highlighted profile when you **save** with `s` in the `p` panel).
- **tab** cycles focus: parameter profiles list → environment → extra arguments. In the profile list, **`n`** adds a profile, **`d`** deletes (not the last one), **enter** renames.
- Storage is a single JSON file under the OS config directory (not environment variables):

  | Platform (typical) | Path                                                                                        |
  | ------------------ | ------------------------------------------------------------------------------------------- |
  | Linux (XDG)        | `$XDG_CONFIG_HOME/llm-launch/model-params.json` or `~/.config/llm-launch/model-params.json` |
  | macOS              | `~/Library/Application Support/llm-launch/model-params.json`                                |
  | Windows            | `%AppData%\llm-launch\model-params.json`                                                    |

  Model rows are keyed by the cleaned absolute filesystem path; each entry stores `profiles` and `activeIndex`.

## Layout

1. `cmd/llm-launch` — `main`, calls `internal/tui.Run()`
2. `internal/tui` — Bubble Tea UI; table via `internal/tui/btable` (fork of Bubbles table)
3. `internal/llamacpp` — GGUF + safetensors discovery, metadata, runtime detection

## Discovery paths

Default roots include `~/models`, `~/.cache/llama.cpp`, `~/.cache/huggingface/hub`,
`~/.cache/lm-studio/models` (only existing directories are scanned).

Add more roots (comma-separated):

```bash
export LLM_LAUNCH_LLAMACPP_PATHS="/data/models,/opt/weights"
```

## llama.cpp binary detection (runtime)

The TUI locates **`llama-cli`** and **`llama-server`** independently and summarizes runtime env vars (paths, optional **`VLLM_VENV`**, listen ports) in the bottom panel. Implementation: `internal/llamacpp` (`DiscoverRuntime`).

**Startup order:** on launch (and on `r` refresh), the app runs **runtime detection** (`llama.cpp` + **vLLM**), then scans for GGUF and safetensors model dirs (`tea.Sequence` in the TUI).

**Per binary** (`llama-cli` and `llama-server` each use this sequence):

1. **`LLAMA_CPP_PATH`** — if set, the file `{LLAMA_CPP_PATH}/<binary-name>` must exist as a regular file.
2. **Common install directories** (first match wins): `/usr/local/bin`, `/opt/homebrew/bin`, `/opt/llama.cpp/build/bin`, `~/.local/bin`.
3. **`PATH`** — `exec.LookPath` (same as a normal shell lookup).

**If both `llama-cli` and `llama-server` are still missing:** the app probes a **running llama.cpp server** with HTTP **GET** `http://127.0.0.1:<port>/health` (2s timeout, success = HTTP 200). Port comes from **`LLAMA_SERVER_PORT`**, or **8080** if unset.

### vLLM binary detection

The **`vllm`** executable is resolved similarly to `llama-server`:

1. **`VLLM_PATH`** — if set, `{VLLM_PATH}/vllm` if that file exists, else **`{VLLM_PATH}/.venv/bin/vllm`** (typical pip/uv install into a project venv).
2. **`VLLM_VENV`** — if set, **`{VLLM_VENV}/bin/vllm`** when present (venv root).
3. **Common install directories:** `/usr/local/bin`, `/opt/homebrew/bin`, `~/.local/bin`.
4. **`PATH`** — `exec.LookPath("vllm")`.

### vLLM Python venv (Linux/macOS)

If vLLM is installed in a virtualenv, **`R`** runs **`. /path/to/activate`** in `sh` before `vllm serve` when a venv activate script is found. Resolution order:

1. **`activate` next to the resolved `vllm` binary** (e.g. `…/.venv/bin/activate` with `…/.venv/bin/vllm`).
2. **`VLLM_VENV`** — venv root, if set (`bin/activate` under it).
3. **`$VLLM_PATH/.venv/bin/activate`** when that file exists.
4. **`dirname(vllm)/.venv/bin/activate`** when `vllm` is a top-level script (e.g. `…/project/vllm` with `…/project/.venv`).

Set **`VLLM_VENV`** explicitly if your layout does not match (2) or (3). On **Windows**, automatic venv activation from **`R`** is not supported; run from an already-activated shell or put `vllm` on `PATH`.

### Listen ports (`LLAMA_SERVER_PORT` and `VLLM_SERVER_PORT`)

Defaults when unset or invalid: **`LLAMA_SERVER_PORT`** **8080** (llama.cpp), **`VLLM_SERVER_PORT`** **8000** (vLLM’s typical default). **`R`** uses **`LLAMA_SERVER_PORT`** for GGUF / `llama-server` and **`VLLM_SERVER_PORT`** for safetensors / `vllm serve`.

| Use                                  | Behavior                                                                            |
| ------------------------------------ | ----------------------------------------------------------------------------------- |
| **`R`** (GGUF row)                   | `llama-server ... --port <LLAMA_SERVER_PORT>`                                       |
| **`R`** (safetensors row)            | `vllm serve ... --port <VLLM_SERVER_PORT>`                                          |
| **Health probe** (no llama binaries) | `GET http://127.0.0.1:<LLAMA_SERVER_PORT>/health` (llama.cpp server only)           |
| **Bottom panel**                     | Five lines: each runtime env var (left) and its value (right), same keys as **`c`** |
| **TUI**                              | **`c`** → edit those five runtime env vars, then **save** with Enter                |

Set ports in the shell or **`mise.toml`** `[env]`. If a default port is busy, pick distinct free ports for llama vs vLLM as needed.

| Variable            | Default  | Role                                                                                 |
| ------------------- | -------- | ------------------------------------------------------------------------------------ |
| `LLAMA_CPP_PATH`    | _(none)_ | Directory containing `llama-cli` / `llama-server`; checked before `PATH`.            |
| `VLLM_PATH`         | _(none)_ | Project directory: we check `vllm` or `.venv/bin/vllm` there before `PATH`.          |
| `VLLM_VENV`         | _(none)_ | Optional Python venv root (`bin/activate`); **`R`** sources it before `vllm` (Unix). |
| `LLAMA_SERVER_PORT` | `8080`   | `llama-server` **`R`**, `/health` probe; editable in **`c`** modal.                  |
| `VLLM_SERVER_PORT`  | `8000`   | `vllm serve` **`R`** for vLLM rows; editable in **`c`** modal.                       |

## Setup

1. `mise install` — Go (see `mise.toml`) and tools (e.g. pre-commit via pipx)
2. `npm ci` or `npm install` — Prettier and markdownlint (formatting / CI)
3. `go mod download`

**Configuration:** the app has **no** `config.toml` and does not load a `.env` file. It reads **environment variables** for runtime paths, ports, and discovery (e.g. `LLAMA_CPP_PATH`, `VLLM_PATH`, `VLLM_VENV`, `LLAMA_SERVER_PORT`, `VLLM_SERVER_PORT`, `LLM_LAUNCH_LLAMACPP_PATHS`). **Parameter profiles** (per-model env + args for **`R`**) are persisted in **`llm-launch/model-params.json`** under the OS user config directory (see **Parameter profiles** above). Set env vars in your shell or under `[env]` in `mise.toml` (see **Discovery paths** and runtime sections above in this file).

## Usage

```bash
mise run run      # go run ./cmd/llm-launch
mise run build    # binary at bin/llm-launch
mise run check    # fmt + vet + prettier + markdownlint + tests
```

## Requirements

- Go: see `mise.toml` (`go = "latest"`)
- Node (LTS): for Prettier / markdownlint via `npm install`

## License

MIT
