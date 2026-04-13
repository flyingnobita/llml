# Changelog

All notable changes to this project will be documented in this file.

Format: `- MMM-DD, YYYY - HH:MM AM/PM TIMEZONE - [Concise summary]`

- Apr-13, 2026 - [TUI: c opens listen-port editor for LLAMA_SERVER_PORT; s save, esc cancel; sets process env for R / llama-server]
- Apr-13, 2026 - [Docs: backlog lives under gitignored dev-docs/BACKLOG.md; AGENTS notes local-only dev-docs]
- Apr-13, 2026 - [Docs: README setup matches env-only config; dev-docs SPECS/PLANS/PRD aligned; .gitignore drops exceptions for removed template files]
- Apr-13, 2026 - [Refactor: remove stale template files; update dev-docs/AGENTS.md; extract constants, innerWidth(), styles; simplify FormatModelFolderDisplay; consolidate mouse wheel cases; add package doc comments]
- Apr-13, 2026 - [R: echo + llama-server one-liner before run; panel shows listen port; README + mise env for LLAMA_SERVER_PORT]
- Apr-13, 2026 - [After R: POSIX sh waits for Enter before restoring TUI so server logs stay readable]
- Apr-13, 2026 - [R runs llama-server for selected GGUF via tea.ExecProcess (terminal log stream); README documents output options]
- Apr-13, 2026 - [Startup: detect llama.cpp before GGUF scan; bottom panel lists llama-cli / llama-server paths]
- Apr-13, 2026 - [Detect llama-cli/llama-server: LLAMA_CPP_PATH, brew bin, PATH, HTTP /health (LLAMA_SERVER_PORT); TUI status]
- Apr-13, 2026 - [Table: btable fork — Selected per cell for correct row colors; Header/Cell PaddingRight]
- Apr-13, 2026 - [Table: stronger selected-row highlight; wheel moves selection; Enter copies full path to clipboard]
- Apr-13, 2026 - [Horizontal scroll bar (█/░); discovery follows symlinked Hub repo dirs — fixes missing models--* trees]
- Apr-13, 2026 - [Horizontal scroll for wide table (←/→, h/l, shift+wheel); HF hub root from HUGGINGFACE_HUB_CACHE / HF_HOME]
- Apr-13, 2026 - [Fix FormatSize unit label (off-by-one: GiB showed as TiB); widen Name column; filter CLIP/mmproj GGUFs]
- Apr-13, 2026 - [Path column: HF hub shows models--* folder only; strip snapshots/hash; else parent dir]
- Apr-13, 2026 - [Path column shows model folder (~ shortened), not full file path]
- Apr-13, 2026 - [llama.cpp v1: scan GGUF paths, table with Name/Path/Size/Last modified/Parameters; refresh key]
- Apr-13, 2026 - [Add Go Bubble Tea TUI scaffold; mise Go latest; project-template base]
- Feb-26, 2026 - 10:58 PM +08 - [Set pre-commit to fix then check; pre-push and CI to check-only]
