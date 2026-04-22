# AGENTS.md — LLM Launcher (llml)

AI coding instructions for this project.

---

## Project Overview

**LLM Launcher** (`llml`) is a terminal UI (TUI) for discovering GGUF and Hugging Face-style
safetensors models on the local filesystem, plus Ollama models via the Ollama API,
and launching `llama-server`, `vllm serve`, or Ollama preload flows for a selected row.

- Language: **Go 1.26+**
- UI framework: **Bubble Tea v2** (`charm.land/bubbletea/v2`) + **Lip Gloss v2** (`charm.land/lipgloss/v2`) + **Bubbles v2** (`charm.land/bubbles/v2`)
- GGUF metadata: `abrander/gguf`
- Tooling: `mise` (tool versions + tasks; includes **GoReleaser** for `mise run goreleaser-check` / lint), `npm` (Prettier + markdownlint only)
- Releases: see [dev-docs/releases-and-packaging.md](dev-docs/releases-and-packaging.md) for GoReleaser, Homebrew (`brews` → `flyingnobita/homebrew-tap`), optional Actions secrets, and maintainer automation. Summary: push a `v*` tag after updating repo-root `VERSION` to match; `.github/workflows/release.yml` publishes GitHub Release archives. Users install via `brew tap flyingnobita/tap` then `brew install llml`.

---

## Source Layout

```text
cmd/llml/            # Binary entrypoint (main.go)
internal/
  config/            # TOML persistence ({UserConfigDir}/llml/config.toml): runtime, discovery cache, [[models]]
  models/            # GGUF + safetensors discovery, metadata, runtime detection, formatting; also Ollama API discovery and HF-hub support. Filesystem discovery uses the `modelSource` interface (`ggufSource`, `safetensorsSource`) and Ollama rows are merged from the daemon API.
  tui/               # Bubble Tea model, update, view, styles, keymaps
scripts/             # gofmt-check.sh, precommit-docs-fix.sh
```

---

## Key Conventions

### Go

- Follow standard Go project layout (`cmd/`, `internal/`).
- All exported types and functions must have doc comments.
- Use `go fmt` / `gofmt` for formatting; CI enforces via `scripts/gofmt-check.sh`.
- Run `go vet ./...` before committing.
- Tests live alongside source (`_test.go`) and run with `go test -race ./...`.

### Bubble Tea pattern

- `Model` in `model.go` is a **coordinator** holding 8 sub-state structs (`layoutState`, `themeState`, `tableState`, `runtimeConfigState`, `paramsState`, `serverPaneState`, `launchPreviewState`, `alertsState`) plus top-level fields (`keys`, `runtime`, `loading`, `lastRunNote`, …). `New()` returns an initialized model. Access state via `m.layout.width`, `m.ui.styles`, `m.table.tbl`, `m.server.running`, `m.preview.focused`, `m.alerts.open`, etc.
- `Init()`, `Update()`, `View()` implement `tea.Model`.
- Messages are defined in `messages.go`; commands in `cmd.go`.
- Key dispatch in `Update` delegates to `handleKey` (idle/modal routing) → `tableNavKeys` (shared bindings for both idle and split-pane table focus: config, params, theme, scroll, copy, sort). Split-pane key handling is in `update_split.go`.
- Layout recalculation lives in `layoutTable()` on `Model`, with helpers `computeBodyHeight` and `applyTableAndLogHeights`. Log h-bar visibility is determined from exact style frame sizes (no guess-and-redo second pass). Table row height is chosen so the full `View()` fits the terminal (Bubble Tea otherwise keeps only the **bottom** lines and clips the header).
- Alert history uses a dedicated bottom `viewport` pane toggled with **`a`**. Active work should use the persistent current-status line; meaningful warnings/errors/lifecycle events should append to alert history instead of relying only on transient footer notes.
- **Server launch** (`run_server.go`): `buildServerSpec` resolves backend-specific launch state into a `serverSpec` value; spec methods `foregroundCmd`, `splitCmd`, `invocationEcho`, `previewLine` generate backend- and platform-specific commands. For Ollama, `R` / `ctrl+R` start `ollama serve` if needed and preload the selected model with `keep_alive: -1` on the shared Ollama host rather than starting a per-model port.
- Theme palettes live in `theme.go` (`DarkTheme`, `LightTheme`; startup via `LLML_THEME`, runtime cycle with **`t`**: dark → light → auto). The transient confirmation is a **compact chip on the title row** (not an extra banner line) so the layout does not jump.
  Lip Gloss styles are built in `styles.go` via `newStyles`. Do not call `lipgloss.NewStyle()` inline
  inside `View()` — extend `Theme` / `newStyles` instead.
- Typed enums (`paramFocus`, `paramConfirm`, `paramEditKind`, `runtimeField`, `tableSortCol`, `runServerMode`) are defined in `constants.go`; use these types, not raw `int`, for state fields.
- Magic numbers belong in `constants.go` (package `tui`).

#### Bubble Tea v2 API notes

- **`View()` return type**: `tea.View` (not `string`). Wrap with `tea.NewView(s)`. Set `v.AltScreen = true` for the full-screen view; do **not** use the removed `tea.WithAltScreen()` program option.
- **Key messages**: `tea.KeyPressMsg` (renamed from `tea.KeyMsg`). Fields are `Code rune` and `Text string` — not `Type`/`Runes`.
- **`textinput` width**: `ti.SetWidth(n)` — not `ti.Width = n`.
- **`viewport` constructor**: `viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))` — not `viewport.New(w, h)`. Setters: `SetWidth` / `SetHeight`.
- **Dark-terminal detection**: `compat.HasDarkBackground` (`charm.land/lipgloss/v2/compat`) is a `bool` variable, not a function. `lipgloss.Color` is a function (`func(string) color.Color`); use `color.Color` (from `image/color`) for `Theme` struct fields.
- **Table selection styling**: The upstream `charm.land/bubbles/v2/table` is used directly (no fork). Selected-row highlighting uses a **background color** on the `Selected` style (`lipgloss.NewStyle().Background(theme.TableSelectedBg)`) so it does not conflict with per-cell foreground styles.

### Configuration

- **On-disk config** lives at **`{UserConfigDir}/llml/config.toml`** (see `internal/config`). It stores **`[runtime]`** (default paths, ports, and Ollama host), **`[discovery]`** (extra model roots and last full-scan time), and **`[[models]]`** (cached discovery rows, including Ollama API rows). **`schema_version`** is reserved for future migrations; migrations should backup via existing write paths before transforming.

- **Updates vs user data:** Release packaging (Homebrew formula, archives) ships **only the `llml` binary** — not the config tree. User data stays under **`{UserConfigDir}/llml/`**. **`backups/`** holds timestamped copies before overwrites (pruned to 10 per logical file); **`.last-run-version`** triggers an extra snapshot of `config.toml` and `model-params.json` when the embedded version changes (skipped for `dev` / empty version). See `internal/userdata`, `internal/fsutil.WriteFileAtomic`.

- **Precedence:** **environment variables override** values from `config.toml`; unset env vars fall back to TOML `default_` keys, then built-in defaults.
- **Startup:** if the cache is valid (`schema_version` matches, at least one cached model path still exists on disk), the UI loads without a full filesystem walk. Otherwise a full scan runs and the file is rewritten.
- **`r`** reloads **`[runtime]`** from `config.toml` and re-runs runtime detection (does not rescan models). **`S`** runs a full model discovery and refreshes **`[[models]]`**.
- Saving the runtime panel (**`c`**) updates the process environment and **best-effort** writes **`[runtime]`** to `config.toml` (failure is non-fatal).

**Runtime** env vars (same keys as **`[runtime]`** in TOML):

| Variable                            | Purpose                                                                                                                       |
| ----------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `LLAMA_CPP_PATH`                    | Directory containing `llama-cli`/`llama-server`                                                                               |
| `VLLM_PATH`                         | Directory containing the `vllm` executable                                                                                    |
| `VLLM_VENV`                         | Optional Python venv root; `R` sources `bin/activate` before `vllm` (Unix)                                                    |
| `LLAMA_SERVER_PORT`                 | TCP port for `llama-server` and `/health` probe (default 8080)                                                                |
| `VLLM_SERVER_PORT`                  | TCP port for `vllm serve` (default 8000)                                                                                      |
| `OLLAMA_PATH`                       | Directory containing the `ollama` executable, or the absolute executable path                                                 |
| `OLLAMA_HOST`                       | Ollama API host (default `127.0.0.1:11434`); `R` / `ctrl+R` ensure the daemon is running there and preload the selected model |
| `LLML_MODEL_PATHS`                  | Extra model search roots (comma-separated); merged with `discovery.extra_model_paths` in TOML for scans                       |
| `HUGGINGFACE_HUB_CACHE` / `HF_HOME` | Hugging Face hub cache location                                                                                               |
| `LLML_THEME`                        | Initial TUI palette (`dark` / `light` / `auto`); **`t`** cycles while running (not in runtime `c` text fields)                |

**Parameter profiles** (per-model extra env + argv for `llama-server`, `vllm`, and backend-specific launch helpers, edited with **`p`**) are **not** in `config.toml`: they are stored in **`{UserConfigDir}/llml/model-params.json`** (see `internal/tui/model_params.go`). Keys are stable model identities: cleaned filesystem paths for local rows, model IDs for Ollama rows. Each entry has named profiles and `activeIndex` for which profile **`R`** uses. In the `p` modal, **`c`** duplicates the highlighted profile (clone env + args).

Set machine-specific env (for example `LLAMA_CPP_PATH`) in `mise.local.toml` (gitignored); keep shared tool/tasks config in `mise.toml`.

### Tasks (mise)

| Task         | Command           |
| ------------ | ----------------- |
| Run          | `mise run run`    |
| Build        | `mise run build`  |
| Format (all) | `mise run format` |
| Lint (all)   | `mise run lint`   |
| Test         | `mise run test`   |
| Full check   | `mise run check`  |

### Docs formatting

Markdown, YAML, and JSON are formatted with **Prettier** and linted with
**markdownlint-cli2**. Run `mise run format` before committing docs changes.
The pre-commit hook handles staged files automatically.

---

## Testing

- Unit tests for `internal/config` cover TOML round-trip, env precedence over
  TOML, cache validation, and stale path filtering.
- Unit tests for `internal/models` cover discovery, formatting, paths, and
  runtime detection.
- Unit tests for `internal/tui` cover model initialization, parameter-profile
  persistence, server command construction, `layoutTable` idempotence (convergence
  test), split-pane focus toggle, launch-preview focus cycle, theme correctness
  (including `TableSelectedBg`), `View()` alt-screen flag, and selected-style background rendering.
- Do not mark a feature complete until `mise run check` passes.

---

## Local-only docs (`dev-docs/`)

The `dev-docs/` directory is gitignored. Use it for notes that should not be
committed (e.g. `dev-docs/BACKLOG.md` for a personal backlog).

## Architecture Decision Records

ADRs live in `dev-docs/adr/YYYYMMDD-short-title.md`; index in
`dev-docs/DECISIONS.md`. Add an ADR for any significant design choice.
