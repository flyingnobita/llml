# AGENTS.md — llm-launch

AI coding instructions for this project.

---

## Project Overview

**llm-launch** is a terminal UI (TUI) for discovering GGUF model files on the
local filesystem and launching `llama-server` for a selected model.

- Language: **Go 1.26+**
- UI framework: **Bubble Tea** (Elm-style TUI) + **Lip Gloss** (styling)
- GGUF metadata: `abrander/gguf`
- Tooling: `mise` (tool versions + tasks), `npm` (Prettier + markdownlint only)

---

## Source Layout

```text
cmd/llm-launch/      # Binary entrypoint (main.go)
internal/
  llamacpp/          # GGUF discovery, metadata, runtime detection, formatting
  tui/               # Bubble Tea model, update, view, styles, keymaps
  tui/btable/        # Vendored fork of charmbracelet/bubbles/table (per-cell Selected)
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
- Layout recalculation lives in `layoutTable()` on `Model`.
- Styles are centralized in `styles.go`. Do not call `lipgloss.NewStyle()` inline
  inside `View()` — add named vars to `styles.go` instead.
- Magic numbers belong in `constants.go` (package `tui`).

### Configuration

Config is entirely **environment-variable-driven** (no config.toml at runtime):

| Variable                            | Purpose                                         |
| ----------------------------------- | ----------------------------------------------- |
| `LLAMA_CPP_PATH`                    | Directory containing `llama-cli`/`llama-server` |
| `LLAMA_SERVER_PORT`                 | TCP port for `llama-server` (default 8080)      |
| `LLM_LAUNCH_LLAMACPP_PATHS`         | Extra GGUF search roots (comma-separated)       |
| `HUGGINGFACE_HUB_CACHE` / `HF_HOME` | Hugging Face hub cache location                 |

Set development defaults in `mise.toml` under `[env]`.

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
- Unit tests for `internal/tui` cover model initialization and server command
  construction.
- `btable` has no separate tests (it is a minimal fork; behavior is covered by
  the TUI tests).
- Do not mark a feature complete until `mise run check` passes.

---

## Local-only docs (`dev-docs/`)

The `dev-docs/` directory is gitignored. Use it for notes that should not be
committed (e.g. `dev-docs/BACKLOG.md` for a personal backlog).

## Architecture Decision Records

ADRs live in `dev-docs/adr/YYYYMMDD-short-title.md`; index in
`dev-docs/DECISIONS.md`. Add an ADR for any significant design choice.
