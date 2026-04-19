# Backlog

Planned work, bugs, and ideas that are **not** shipped yet. Shipped changes belong in [CHANGELOG.md](../CHANGELOG.md).

**Detailed tracking:** [GitHub Issues](https://github.com/flyingnobita/llml/issues).

Use **one line per item**. Link an issue when it exists (`#42`). Drop checkboxes if you prefer plain bullets.

Recommended format for items:

- [ ] (Component(s) related to item) Description

## Done

Completed backlog items (recent).

- [x] (TUI) Listen port for `LLAMA_SERVER_PORT` (`c` to open, Enter to save, `esc` to cancel)
- [x] (vLLM) Safetensors + `config.json` dirs discovered; `R` runs `vllm serve`; `VLLM_PATH` + detection
- [x] (Config) TOML persistence at `{UserConfigDir}/llml/config.toml` — `schema_version`, `[runtime]`, `[discovery]`, `[[models]]`; env overrides TOML; skip full scan when cache valid; `r` reload runtime only, `S` rescan models, `c` write `[runtime]`; merge `discovery.extra_model_paths` with `LLML_MODEL_PATHS`; last-scan timestamp in UI; warn on config write failure
- [x] (Parameter Profiles) Persist an active profile per model and use it for `R`, `ctrl+R`, and launch preview

## Now

Work you intend to do soon.

- [ ] (Runtime Detection) Warn user if no runtimes are detected
- [ ] (Model Detection) Warn user if no models are detected

## Next

Queued after “Now,” or medium priority.

- [ ] (Parameter Profiles) Allow user to have optional overrides for runtime configuration in parameter profiles
- [ ] (CLI) Add CLI to run servers with parameter profiles
- [ ] (Platform) macOS support
- [ ] (Platform) Windows WSL support
- [ ] (TUI) refactor hint bar across windows

## Later

Low priority or blocked.

- [ ]

## Ideas

Exploratory; may never ship.

- [ ]

## Known issues (quick refs)

Short reminders; full write-ups stay in Issues.

- [ ]
