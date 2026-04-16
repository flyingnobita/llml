# AGENTS.md — LLM Launcher (llml)

AI coding instructions for this project.

---

## Project Overview

**LLM Launcher** (`llml`) is a terminal UI (TUI) for discovering GGUF and Hugging Face-style
safetensors models on the local filesystem and launching `llama-server` or
`vllm serve` for a selected row.

- Language: **Go 1.26+**
- UI framework: **Bubble Tea v2** (`charm.land/bubbletea/v2`) + **Lip Gloss v2** (`charm.land/lipgloss/v2`) + **Bubbles v2** (`charm.land/bubbles/v2`)
- GGUF metadata: `abrander/gguf`
- Tooling: `mise` (tool versions + tasks), `npm` (Prettier + markdownlint only)
- Releases: [GoReleaser](https://goreleaser.com/) (`.goreleaser.yaml`); pushing a `v*` tag runs `.github/workflows/release.yml` and publishes archives to GitHub Releases

---

## Source Layout

```text
cmd/llml/            # Binary entrypoint (main.go)
internal/
  llamacpp/          # GGUF + safetensors discovery, metadata, runtime detection, formatting
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

- `Model` in `model.go` holds all state. `New()` returns an initialized model.
- `Init()`, `Update()`, `View()` implement `tea.Model`.
- Messages are defined in `messages.go`; commands in `cmd.go`.
- Layout recalculation lives in `layoutTable()` on `Model`. Table row height is chosen so the full `View()` fits the terminal (Bubble Tea otherwise keeps only the **bottom** lines and clips the header).
- Theme palettes live in `theme.go` (`DarkTheme`, `LightTheme`; startup via `LLML_THEME`, runtime cycle with **`t`**: dark → light → auto). The transient confirmation is a **compact chip on the title row** (not an extra banner line) so the layout does not jump.
  Lip Gloss styles are built in `styles.go` via `newStyles`. Do not call `lipgloss.NewStyle()` inline
  inside `View()` — extend `Theme` / `newStyles` instead.
- Magic numbers belong in `constants.go` (package `tui`).

#### Bubble Tea v2 API notes

- **`View()` return type**: `tea.View` (not `string`). Wrap with `tea.NewView(s)`. Set `v.AltScreen = true` for the full-screen view; do **not** use the removed `tea.WithAltScreen()` program option.
- **Key messages**: `tea.KeyPressMsg` (renamed from `tea.KeyMsg`). Fields are `Code rune` and `Text string` — not `Type`/`Runes`.
- **`textinput` width**: `ti.SetWidth(n)` — not `ti.Width = n`.
- **`viewport` constructor**: `viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))` — not `viewport.New(w, h)`. Setters: `SetWidth` / `SetHeight`.
- **Dark-terminal detection**: `compat.HasDarkBackground` (`charm.land/lipgloss/v2/compat`) is a `bool` variable, not a function. `lipgloss.Color` is a function (`func(string) color.Color`); use `color.Color` (from `image/color`) for `Theme` struct fields.
- **Table selection styling**: The upstream `charm.land/bubbles/v2/table` is used directly (no fork). Selected-row highlighting uses a **background color** on the `Selected` style (`lipgloss.NewStyle().Background(theme.TableSelectedBg)`) so it does not conflict with per-cell foreground styles.

### Configuration

**Runtime** config is **environment-variable-driven** (no `config.toml` at runtime):

| Variable                            | Purpose                                                                                                        |
| ----------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `LLAMA_CPP_PATH`                    | Directory containing `llama-cli`/`llama-server`                                                                |
| `VLLM_PATH`                         | Directory containing the `vllm` executable                                                                     |
| `VLLM_VENV`                         | Optional Python venv root; `R` sources `bin/activate` before `vllm` (Unix)                                     |
| `LLAMA_SERVER_PORT`                 | TCP port for `llama-server` and `/health` probe (default 8080)                                                 |
| `VLLM_SERVER_PORT`                  | TCP port for `vllm serve` (default 8000)                                                                       |
| `LLML_MODEL_PATHS`                  | Extra model search roots (comma-separated)                                                                     |
| `HUGGINGFACE_HUB_CACHE` / `HF_HOME` | Hugging Face hub cache location                                                                                |
| `LLML_THEME`                        | Initial TUI palette (`dark` / `light` / `auto`); **`t`** cycles while running (not in runtime `c` text fields) |

**Parameter profiles** (per-model extra env + argv for `llama-server` / `vllm`, edited with **`p`**) are **not** env vars: they are stored in **`{UserConfigDir}/llml/model-params.json`** (see `internal/tui/model_params.go`). Keys are cleaned model paths; each entry has named profiles and `activeIndex` for which profile **`R`** uses.

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

- Unit tests for `internal/llamacpp` cover discovery, formatting, paths, and
  runtime detection.
- Unit tests for `internal/tui` cover model initialization, parameter-profile
  persistence, server command construction, theme correctness (including
  `TableSelectedBg`), `View()` alt-screen flag, and selected-style background rendering.
- Do not mark a feature complete until `mise run check` passes.

---

## Local-only docs (`dev-docs/`)

The `dev-docs/` directory is gitignored. Use it for notes that should not be
committed (e.g. `dev-docs/BACKLOG.md` for a personal backlog).

## Architecture Decision Records

ADRs live in `dev-docs/adr/YYYYMMDD-short-title.md`; index in
`dev-docs/DECISIONS.md`. Add an ADR for any significant design choice.
