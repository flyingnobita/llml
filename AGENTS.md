# AGENTS.md / CLAUDE.md — LLM Launcher (llml)

AI coding instructions for this project.

This instruction lives in `AGENTS.md`. Update it here, not in `CLAUDE.md`.

---

## Project Overview

**LLM Launcher** (`llml`) is a terminal UI (TUI) for discovering GGUF and Hugging Face-style
safetensors models on the local filesystem, plus Ollama models via the Ollama API,
and launching `llama-server`, `koboldcpp`, `vllm serve`, or Ollama preload flows for a selected row.

- Language: **Go 1.26+**
- UI framework: **Bubble Tea v2** (`charm.land/bubbletea/v2`) + **Lip Gloss v2** (`charm.land/lipgloss/v2`) + **Bubbles v2** (`charm.land/bubbles/v2`)
- GGUF metadata: `abrander/gguf`
- Tooling: `mise` (tool versions + tasks; includes **GoReleaser** for `mise run goreleaser-check` / lint), `npm` (Prettier + markdownlint only)
- Releases: see [dev-docs/llml/releases-and-packaging.md](dev-docs/llml/releases-and-packaging.md) for GoReleaser, Homebrew (`brews` -> `flyingnobita/homebrew-tap`), Scoop (`scoops` -> `flyingnobita/scoop-bucket`), Winget (`winget` -> `flyingnobita/winget-pkgs` -> `microsoft/winget-pkgs` PR), required secrets, and maintainer automation. Summary: push a `v*` tag after updating repo-root `VERSION` to match; `.github/workflows/release.yml` publishes GitHub Release archives. Users install via `brew tap flyingnobita/tap && brew install --cask llml`, `scoop bucket add flyingnobita https://github.com/flyingnobita/scoop-bucket && scoop install flyingnobita/llml`, or `winget install --id FlyingNobita.llml` after the Winget PR merges.
- **Profile catalog (separate repo):** [`flyingnobita/llml-catalog`](https://github.com/flyingnobita/llml-catalog) holds the community TOML profile store and its static index site. Work on the catalog, site, and design system belongs in that repo, not here. The portable profile schema defined in `docs/profile-format.md` is the one-way contract between the two repos.

---

## Agent coordination

Before starting work, read the live working board at
`/home/omarchy/Data/Projects/Personal/llml-internal/BOARD_working.md`
to see what tasks are already claimed by other agents. Claim a task by moving it from
"Up next" → "In progress" with your agent ID. Move it to "Recently completed" when done.

`BOARD_working.md` is gitignored in llml-internal — changes are real-time and shared across
both repos via the same absolute path. Do NOT commit it. At end of session, the user
archives it to `BOARD.md`.

---

---

## Source Layout

```text
cmd/llml/            # Binary entrypoint (main.go)
internal/
  settings/          # Resolved runtime configuration: Settings, Layer, Resolve, FromEnv, Defaults. Owns the env var names and built-in defaults; imports nothing from config/models/tui
  config/            # TOML persistence ({UserConfigDir}/llml/config.toml): runtime, discovery cache, [[models]]. Converts to and from settings via RuntimeConfig.Layer / RuntimeConfigFromSettings
  models/            # GGUF + safetensors discovery, metadata, runtime detection, formatting; also Ollama API discovery and HF-hub support. Filesystem discovery uses the `modelSource` interface (`ggufSource`, `safetensorsSource`) and Ollama rows are merged from the daemon API.
  profiles/          # Parameter profiles end to end: model-params.json persistence (Entry, Profile, ModelParams), portable TOML parse/write (portable_parse.go), and the single model-location rule table (model_location.go). internal/tui uses these types directly — do not reintroduce alias wrappers
  tui/               # Bubble Tea model, update, view, styles, keymaps
.agents/skills/     # Canonical repo-managed agent skills; llml-import lives here
.claude/skills/     # Tracked Claude compatibility copies for repo-managed skills
scripts/             # gofmt-check.sh, precommit-docs-fix.sh
```

---

## Key Conventions

### Go

- Follow standard Go project layout (`cmd/`, `internal/`).
- All exported types and functions must have doc comments.
- Use `go fmt` / `gofmt` for formatting; CI enforces via `scripts/gofmt-check.sh`.
- Run `go vet ./...` before committing.
- **Never call `os.Setenv` / `os.Unsetenv` in production code, and do not read
  `os.Getenv` outside `internal/settings/env.go`** (the two exceptions are
  `internal/tui/debug.go` and `internal/tui/theme.go`). Runtime values are
  resolved once into a `settings.Settings` and passed explicitly; see the ADR
  `dev-docs/llml/adr/20260907-resolved-settings-instead-of-env.md`. Tests pass
  a fake `settings.Getenv` rather than mutating the process environment, which
  is what lets them use `t.Parallel`.
- Tests live alongside source (`_test.go`) and run with `go test -race ./...`.

### Bubble Tea pattern

- `Model` in `model.go` is a **coordinator** holding 11 sub-state structs (`layoutState`, `themeState`, `tableState`, `runtimeConfigState`, `paramsState`, `serverPaneState`, `launchPreviewState`, `alertsState`, `discoveryPathsState`, `exportViewState`, `collisionState`) plus top-level fields (`keys`, `runtime`, `loading`, `lastRunNote`, …). `New()` returns an initialized model. Access state via `m.layout.width`, `m.ui.styles`, `m.table.tbl`, `m.server.running`, `m.preview.focused`, `m.alerts.open`, `m.export.open`, etc.
- `Init()`, `Update()`, `View()` implement `tea.Model`.
- Messages are defined in `messages.go`; commands in `cmd.go`.
- Key dispatch in `Update` delegates to `handleKey` (idle/modal routing) → `tableNavKeys` (shared bindings for both idle and split-pane table focus: config, params, theme, scroll, copy, sort). Split-pane key handling is in `update_split.go`.
- Layout recalculation lives in `layoutTable()` on `Model`, with helpers `computeBodyHeight` and `applyTableAndLogHeights`. Log h-bar visibility is determined from exact style frame sizes (no guess-and-redo second pass). Table row height is chosen so the full `View()` fits the terminal (Bubble Tea otherwise keeps only the **bottom** lines and clips the header).
- Alert history uses a dedicated bottom `viewport` pane toggled with **`a`**. Active work should use the persistent current-status line; meaningful warnings/errors/lifecycle events should append to alert history instead of relying only on transient footer notes.
- **Value semantics:** every method on `Model` uses a **value receiver** and returns a new `Model` — there are no pointer receivers. Because a copied `Model` shares its maps and slice backing arrays, any field mutated after construction is **copy-on-write**: clone before writing (`maps.Clone` for `table.effectiveBackends`, `withExportItemsCloned` / `withImportGroupsCloned` for the item slices). Appending is fine; writing `slice[i]` on a shared array is not.
- **Cancellation:** every function that performs network or subprocess I/O takes a `context.Context` as its first parameter. `tea.Cmd`s that do I/O create one from a timeout in `constants.go` (`scanTimeout`, `runtimeProbeTimeout`, `ollamaPreloadTimeout`); a scan's `CancelFunc` is stored on `Model.scanCancel` by `startScan` and released by `cancelInFlightScan` on quit or when the scan's result arrives. `models.DiscoverRuntime` runs its three probes concurrently, so an unreachable backend costs one timeout rather than three.
- **`models.Discover` is filesystem-only.** It never calls the network; Ollama rows come from `models.OllamaClient.Tags` and are merged by the caller. Do not reintroduce API calls into the walk — a hung daemon must not be able to stall a filesystem scan.
- **HTTP:** `internal/models` shares one package-level client (`httpclient.go`) so repeated probes reuse connections, and `internal/profiles` has its own via `Fetcher`. Neither sets a client `Timeout`: per-call deadlines come from the context. Do not construct an `http.Client` per call.
- **External dependencies** (`services.go`): the config file, discovery, the Ollama daemon, the environment reader, and the clipboard are collected in a `services` struct held on `Model.svc`. `New()` wires the real ones via `defaultServices()`; `NewWithServices(svc)` injects fakes. Command builders (`startupCmd`, `applyAndFullScanCmd`, `rescanModelsCmd`, `reloadRuntimeCmd`, `runOllamaLaunchCmd`) are methods on `services`. **Do not add package-level function variables as test seams** — add a field to `services` instead, so the dependency is visible and tests stay parallel-safe.
- **Server launch** (`run_server.go`): `buildServerSpec` resolves backend-specific launch state into a `serverSpec` value; spec methods `foregroundCmd`, `splitCmd`, `invocationEcho`, `previewLine` generate backend- and platform-specific commands. For KoboldCpp, the effective backend comes from the active parameter profile and GGUF rows can switch between llama.cpp and KoboldCpp. For Ollama, `R` / `ctrl+R` start `ollama serve` if needed and preload the selected model with `keep_alive: -1` on the shared Ollama host rather than starting a per-model port. **mmproj injection (opt-in):** for `llama` and `koboldcpp` backends, `buildServerSpec` injects `--mmproj` only when the active profile has `"image"` or `"audio"` in `use_case.tags` (`profileWantsMMProj`). When opted in, `models.ResolveMMProj` looks for a sibling `*mmproj*.gguf` in the model's directory; if exactly one is found (or one is unambiguously identified by a shared file-name prefix), `--mmproj <path>` is injected into all argv builders (`directArgs`, `commandWords`) and appears in the launch preview and clipboard output. When multiple mmproj files are found and disambiguation fails, `mmprojCandidates` is set and a warn alert is emitted at launch. When the profile has the tag but no mmproj file is found, `mmprojMissing` is set, a warn alert is emitted at launch, and the launch preview pane shows a warning via `mmprojNote()`. The `serverSpec` fields `mmprojPath`, `mmprojCandidates`, and `mmprojMissing` carry this state. mmproj resolution is skipped when the active profile already contains a `--mmproj` / `-mm` arg token. Image/Audio can be toggled per-profile in the `p` panel (visible only for `llama`/`koboldcpp` backends).
- Theme palettes live in `theme.go` (`DarkTheme`, `LightTheme`; startup via `LLML_THEME`, runtime cycle with **`t`**: dark → light → auto). The transient confirmation is a **compact chip on the title row** (not an extra banner line) so the layout does not jump.
  Lip Gloss styles are built in `styles.go` via `newStyles`. **Never call `lipgloss.NewStyle()` outside `styles.go`** — `forbidigo` enforces this. An inline style ignores the active theme, so `t` would not restyle it. Extend `Theme` / `newStyles` instead, and take the width or colour that varies per call site as an argument on the pre-built style (`m.ui.styles.helpKey.Width(n)`). A widget that keeps its own copy of its styles (the Notes `textarea`) must also be re-pushed in `cycleTheme`.
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

- **On-disk config** lives at **`{UserConfigDir}/llml/config.toml`** (see `internal/config`). It is **user-owned** and stores only **`[runtime]`** (default paths, ports, and hosts) and **`[discovery].extra_model_paths`**. llml rewrites it **only when the user saves** from the **`c`** or model-paths panels — never as a side effect of a scan, so hand edits survive discovery.

- **Discovery cache** lives at **`{UserConfigDir}/llml/cache/models.toml`** (`internal/config/cache.go`: `CacheFile`, `ReadCache`, `WriteCache`). It is **machine-owned and disposable**: `schema_version`, `last_scan`, and `[[models]]` (including Ollama API rows). Deleting it costs one rescan, so it is not backed up. **Do not move machine state back into `config.toml`.**

- **Schema migration:** `config.toml` is at **`schema_version = 4`**. Version 3 kept the model cache inline; `ReadFile` migrates such a file once, moving `[[models]]` and `last_scan` into the cache and rewriting `config.toml` (the previous file is snapshotted under `backups/` first). Only version 3 is migrated — older schemas stored rows in shapes the cache cannot trust, and their models are discarded in favor of a rescan. A migration that cannot write is logged and retried next read, never fatal. Note that the one-time rewrite does **not** preserve comments, because the TOML encoder cannot; ordinary scans leave the file untouched byte for byte.

- **Updates vs user data:** Release packaging (Homebrew cask, archives) ships **only the `llml` binary** — not the config tree. User data stays under **`{UserConfigDir}/llml/`**. **`backups/`** holds timestamped copies before overwrites (pruned to 10 per logical file); **`.last-run-version`** triggers an extra snapshot of `config.toml` and `model-params.json` when the embedded version changes (skipped for `dev` / empty version). See `internal/userdata`, `internal/fsutil.WriteFileAtomic`.

- **Precedence:** **environment variables override** values from `config.toml`; unset env vars fall back to TOML `default_` keys, then built-in defaults. This is resolved **once, at startup**, by `settings.Resolve(settings.FromEnv(getenv), cfg.Layer(), settings.Defaults())` — the first layer that sets a field wins. Changing an environment variable after llml starts has no effect.
- **Startup:** if the cache is valid (`schema_version` matches, at least one cached model path still exists on disk), the UI loads without a full filesystem walk. Otherwise a full scan runs and the file is rewritten.
- **`r`** reloads **`[runtime]`** from `config.toml` and re-runs runtime detection (does not rescan models). **`S`** runs a full model discovery and refreshes **`[[models]]`**.
- Saving the runtime panel (**`c`**) builds a new `settings.Settings` from the inputs, assigns it to the model, re-runs runtime detection, and **best-effort** writes **`[runtime]`** to `config.toml` (failure is non-fatal). An empty port or host field means "use the built-in default".

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
| `KOBOLDCPP_PATH`                    | Directory containing the `koboldcpp` executable, or the absolute executable path                                              |
| `KOBOLDCPP_PORT`                    | TCP port for KoboldCpp and `/api/extra/generate/check` health probe (default 5001)                                            |
| `LLML_MODEL_PATHS`                  | Extra model search roots (comma-separated); merged with `discovery.extra_model_paths` in TOML for scans                       |
| `HUGGINGFACE_HUB_CACHE` / `HF_HOME` | Hugging Face hub cache location                                                                                               |
| `LLML_THEME`                        | Initial TUI palette (`dark` / `light` / `auto`); **`t`** cycles while running (not in runtime `c` text fields)                |

**Parameter profiles** (per-model extra env + argv for `llama-server`, `koboldcpp`, `vllm`, and backend-specific launch helpers, edited with **`p`**) are **not** in `config.toml`: they are stored in **`{UserConfigDir}/llml/model-params.json`** (see `internal/profiles`). Keys are stable model identities: cleaned filesystem paths for local rows, model IDs for Ollama rows. Each entry has named profiles and `activeIndex` for which profile **`R`** uses. In the `p` modal, **`c`** duplicates the highlighted profile (clone env + args). The Profile Metadata section has two checkbox rows: **Use Case Primary** (multi-select from `chat`, `tool-calling`, `eval` — left/right moves the chip cursor, space toggles) and **Tags** (same interaction, from a canonical tag list). **Backend** and **Hardware Class** are rendered as single-select radio rows (`( )`/`(•)` chips) — left/right moves the cursor, space/enter selects the highlighted option. All text fields (GPU Count, Min/Max VRAM GB, **Notes** — formerly "Hardware Notes") open a text-input edit on enter. Labels and values use distinct colors; all value columns align at the same horizontal offset regardless of label width.

**Portable profile format** lives in **`docs/profile-format.md`**. Import is built-in:
use **`I`** in the TUI (filepicker + text input) or **`llml import <file.toml>`** from
the CLI. The canonical `llml-import` agent skill at
**`.agents/skills/llml-import/SKILL.md`** remains available for web-scraping new
profiles into portable TOML. Keep that `.agents` file as the source of truth. The
tracked Claude workspace copy at **`.claude/skills/llml-import/SKILL.md`** must stay
byte-for-byte in sync; refresh it with
**`./scripts/sync-skill --workspace --tool claude`** after editing the canonical
skill. User-level installs for local agent tools are handled by **`scripts/sync-skill`**.

Set machine-specific env (for example `LLAMA_CPP_PATH`) in `mise.local.toml` (gitignored); keep shared tool/tasks config in `mise.toml`. For a **linked git worktree**, run **`mise run worktree-setup`** in that checkout so dependencies install and **`scripts/sync_gitignore_agents.sh`** runs with **`LLML_AGENTS_SYNC=import`**, pulling gitignored paths listed at the top of that script (agent dirs, `TODOS.md`, `mise.local.toml`) from the **primary** checkout by default; set **`LLML_AGENTS_PEER`** or **`LLML_AGENTS_SYNC`** (`import` / `export` / `none`) to override.

### Tasks (mise)

| Task                                           | Command                   |
| ---------------------------------------------- | ------------------------- |
| Run                                            | `mise run run`            |
| Build                                          | `mise run build`          |
| Format (all)                                   | `mise run format`         |
| Lint (all)                                     | `mise run lint`           |
| Test                                           | `mise run test`           |
| Full check                                     | `mise run check`          |
| Sync submodules (to parent pin)                | `mise run sync`           |
| Pull latest (`origin/main` + submodule remote) | `mise run pull-latest`    |
| New worktree (deps + import ignored paths)     | `mise run worktree-setup` |

### Toolchain pinning

Go, GoReleaser, and golangci-lint are pinned to exact versions in `mise.toml`;
`.github/workflows/ci.yml` and `release.yml` both resolve them from that one
file. Two constraints must hold when bumping any of them:

- `go` in `mise.toml` matches the `go` directive in `go.mod`. `mise run lint`
  runs `scripts/go-version-check.sh`, which fails the build on drift.
- `github:golangci/golangci-lint` is a release built against that Go version.
  A linter built with an older Go panics on packages compiled by a newer one
  (`file requires newer Go version`). Check with `golangci-lint --version`.

Bump Go by editing `go.mod` and `mise.toml` together, then run `mise install`.

### Lint thresholds

`.golangci.yml` enables `nolintlint`, `errorlint`, `revive`, `gocognit`,
`funlen`, `unparam`, `copyloopvar`, and `forbidigo` on top of the standard set
plus `gosec`, `gocritic`, and `misspell`.

- **Cognitive complexity** (`gocognit`) is capped at **30**, and function length
  (`funlen`) at **120 lines / 60 statements**. When a key handler or renderer
  crosses either, split it by the thing it is switching on: one function per
  focus, per message group, or per row kind.
- **Every `//nolint` must name its linter and give a reason** (`nolintlint`
  enforces both). Do not add one to make a step green — fix the code, or stop
  and ask.
- Tests are exempt from `unparam`, `gocognit`, and `funlen`: a helper's
  parameters are part of its shape, and table-driven tests are legitimately long.
- Narrow, documented exceptions live in the `exclusions` block, not inline.

### Docs formatting

Markdown, YAML, and JSON are formatted with **Prettier** and linted with
**markdownlint-cli2**. Run `mise run format` before committing docs changes.
The pre-commit hook handles staged files automatically.

### Release packaging

- Tagged releases use GoReleaser via `.github/workflows/release.yml`.
- Homebrew publishes to `flyingnobita/homebrew-tap`.
- Scoop publishes to `flyingnobita/scoop-bucket`.
- Winget publishes manifests to `flyingnobita/winget-pkgs` and opens a PR into `microsoft/winget-pkgs`.
- `WINGET_GITHUB_TOKEN` must be a **classic PAT**. A fine-grained PAT can push to the fork but cannot complete the upstream PR creation step.
- Before calling a release done, verify GitHub Release assets published, Homebrew tap updated, Scoop bucket updated, and Winget PR created or manually completed if automation failed.
- Do not leave release notes as a raw autogenerated commit list when the release has meaningful user-facing changes. Edit the GitHub Release body so it summarizes the actual major changes since the previous tag.

---

## Testing

- Unit tests for `internal/settings` cover the precedence rule (env over TOML
  over defaults), value normalization, and invalid-value fallthrough, using a
  map-backed `Getenv`.
- Unit tests for `internal/config` cover TOML round-trip, the settings layer
  mapping and its round trip, cache validation, and stale path filtering.
- Unit tests for `internal/models` cover discovery, formatting, paths, and
  runtime detection.
- Unit tests for `internal/tui` build a `services` value with fakes
  (`testServices()` plus `NewWithServices`) rather than mutating package state,
  and cover model initialization, parameter-profile
  persistence, server command construction, `layoutTable` idempotence (convergence
  test), split-pane focus toggle, launch-preview focus cycle, theme correctness
  (including `TableSelectedBg`), `View()` alt-screen flag, and selected-style background rendering.
- Unit tests for `scripts/` cover agent-skill sync behavior, including workspace/user
  install flows and parity between the canonical `.agents` skill and the tracked
  Claude compatibility copy.
- **Golden files** (`internal/tui/testdata/*.golden`) pin the rendered layout of
  the main view, the runtime panel, the help panel, and the alert pane. They
  catch shifts an assertion-based test misses (a column that moves, a title that
  truncates). Regenerate with `go test ./internal/tui -run Golden -update` and
  **read the diff** — a changed golden file is either the change you intended or
  a regression. ANSI styling is stripped and the alert clock is blanked, so
  theme and time do not churn them.
- Keep test files focused: when one passes roughly 500 lines, split it by
  behavior (see `internal/tui/export_view_*_test.go`).
- Do not mark a feature complete until `mise run check` passes.
- **Platform portability:** When tests control `os.UserConfigDir()` or
  `os.UserHomeDir()`, set the relevant env vars (`HOME`, `XDG_CONFIG_HOME`)
  and call the stdlib function to resolve the path — never hardcode
  platform-specific directories like `Library/Application Support` or `.config`.
  CI runs Linux, but development happens on macOS; hardcoded paths pass
  locally and fail in CI.

---

## Local-only docs (`dev-docs/llml/`)

The `dev-docs/llml/` directory is gitignored. Use it for notes that should not be
committed (e.g. `dev-docs/llml/BACKLOG.md` for a personal backlog).

## Architecture Decision Records

ADRs live in `dev-docs/llml/adr/YYYYMMDD-short-title.md`; index in
`dev-docs/llml/DECISIONS.md`. Add an ADR for any significant design choice.

## GBrain Configuration (configured by /setup-gbrain)

- Mode: local-stdio
- Engine: postgres
- Config file: ~/.gbrain/config.json (mode 0600)
- Setup date: 2026-05-07
- MCP registered: yes
- Artifacts sync: full
- Current repo policy: read-write
- To Do list: instead of `TODOS.md`, use `dev-docs/llml/BACKLOG.md`

### GBrain Search Guidance (configured by /sync-gbrain)

<!-- gstack-gbrain-search-guidance:start -->

GBrain is set up and synced on this machine. The agent should prefer gbrain
over Grep when the question is semantic or when you don't know the exact
identifier yet. Two indexed corpora available via the `gbrain` CLI:

- This repo's code (registered as `gstack-code-<repo>` source).
- `~/.gstack/` curated memory (registered as `gstack-brain-<user>` source via
  the existing federation pipeline).

Prefer gbrain when:

- "Where is X handled?" / semantic intent, no exact string yet:
  `gbrain search "<terms>"` or `gbrain query "<question>"`
- "Where is symbol Y defined?" / symbol-based code questions:
  `gbrain code-def <symbol>` or `gbrain code-refs <symbol>`
- "What calls Y?" / "What does Y depend on?":
  `gbrain code-callers <symbol>` / `gbrain code-callees <symbol>`
- "What did we decide last time?" / past plans, retros, learnings:
  `gbrain search "<terms>" --source gstack-brain-<user>`

Grep is still right for known exact strings, regex, multiline patterns, and
file globs. The brain auto-syncs incrementally on every gstack skill start.
Run `/sync-gbrain` to force-refresh, `/sync-gbrain --full` for full reindex.

<!-- gstack-gbrain-search-guidance:end -->

### Skill routing

When the user's request matches an available skill, invoke it via the Skill tool. When in doubt, invoke the skill.

Key routing rules:

- Product ideas/brainstorming → invoke /office-hours
- Strategy/scope → invoke /plan-ceo-review
- Architecture → invoke /plan-eng-review
- Design system/plan review → invoke /design-consultation or /plan-design-review
- Full review pipeline → invoke /autoplan
- Bugs/errors → invoke /investigate
- QA/testing site behavior → invoke /qa or /qa-only
- Code review/diff check → invoke /review
- Visual polish → invoke /design-review
- Ship/deploy/PR → invoke /ship or /land-and-deploy
- Save progress → invoke /context-save
- Resume context → invoke /context-restore

## Health Stack

- typecheck: go vet ./...
- lint: scripts/gofmt-check.sh && golangci-lint run ./...
- test: go test -race ./...
- umbrella: mise run check
- gbrain: timeout 5s gbrain doctor --json

## Design System

Always read `dev-docs/llml/DESIGN.md` before making any visual or UI decisions.
All font choices, colors, spacing, and aesthetic direction are defined there.
Do not deviate without explicit user approval.
In QA mode, flag any code that doesn't match `dev-docs/llml/DESIGN.md`.
