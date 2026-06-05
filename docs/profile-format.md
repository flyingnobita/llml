# Portable Parameter Profile Format

Date: 2026-05-01
Status: Proposed

## Purpose

This document defines the portable parameter profile format for llml. It serves
three purposes:

1. **Human documentation** - describes what a shareable profile file looks like and
   how to write one by hand.
2. **Machine prompt context** - an LLM pointed at this document and a source URL
   (model card, blog post, README) can extract structured profiles without
   additional instructions.
3. **Export contract** - `llml export` (TUI `E` or CLI) writes profiles in this
   same format so you can share, review, or import them on another machine.

## Background

llml stores parameter profiles per model in `{UserConfigDir}/llml/model-params.json`.
Each profile has a name, environment variables, command-line args, and structured
metadata for backend, use case, and hardware expectations. That internal format is
not portable - it is keyed by local model path and not designed for sharing.

The portable profile format defined here is a separate, self-contained TOML file
intended for sharing — both importing and exporting. Import tooling (for example
the `/llml-import` agent skill) reads this format, maps its metadata into llml's
canonical local profile schema, and writes the resulting profiles into
`model-params.json`. The `E` key or `llml export` CLI writes profiles from
`model-params.json` back into this same portable format.

## Scope

- Covers llama.cpp, vLLM, Ollama, and KoboldCpp backends.
- One file may contain multiple profiles, for multiple backends, targeting one or
  more model families.
- Cross-backend translation (for example converting a llama.cpp profile to a
  KoboldCpp or Ollama equivalent) is out of scope for this format version.

## Schema

### Top-level fields

| Field            | Type    | Required | Description                   |
| ---------------- | ------- | -------- | ----------------------------- |
| `schema_version` | integer | yes      | Must be `3` for this version. |

### `[[profiles]]` array

Each entry in `[[profiles]]` is one parameter profile.

| Field        | Type            | Required | Description                                                                                                                |
| ------------ | --------------- | -------- | -------------------------------------------------------------------------------------------------------------------------- |
| `name`       | string          | yes      | Short human-readable name for this profile. Used as the profile name inside llml.                                          |
| `backend`    | string          | yes      | One of: `llama`, `vllm`, `ollama`, `koboldcpp`. Semantics match llml's local canonical profile schema.                     |
| `model_hint` | string          | no       | Free-text hint for which local model these profiles should attach to (for example `"Llama-3-8B-GGUF"` or `"Qwen2.5-72B"`). |
| `args`       | array of string | no       | Command-line arguments in panel-row format (see below). Defaults to empty.                                                 |
| `env`        | array of table  | no       | Environment variables as `{key, value}` pairs. Defaults to empty.                                                          |
| `use_case`   | table           | no       | Structured profile purpose metadata. Semantics match llml's local canonical profile schema after import.                   |
| `hardware`   | table           | no       | Structured hardware compatibility metadata. Semantics match llml's local canonical profile schema after import.            |

### `[profiles.use_case]` format

The `use_case` table maps to llml's local `useCase` object.

| Field     | Type            | Required | Description                                                                                                                                                                |
| --------- | --------------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `primary` | array of string | no       | One or more of: `chat`, `tool-calling`, `eval`.                                                                                                                            |
| `tags`    | array of string | no       | Normalized tags. Two canonical behavioral tags control mmproj injection: `"image"` and `"audio"` (see below). Other common tags: `interactive`, `low-latency`, `balanced`. |

Normalization rules:

- `primary` is an array; use `["chat"]` for a single value and `["chat",
"tool-calling"]` for multiple. An empty array or omitted field means unspecified.
- Canonical primary values are `chat`, `tool-calling`, and `eval`.
- Importers normalize synonyms: `assistant → chat`, `tool_calling → tool-calling`,
  `evaluation → eval`. Unknown values are silently dropped on import.
- Tags should be lowercase and kebab-case where practical.

**Canonical behavioral tags — `"image"` and `"audio"`:** When a profile for a
`llama` or `koboldcpp` backend includes `"image"` or `"audio"` in `use_case.tags`,
llml will attempt to inject `--mmproj <path>` at launch using a sibling
`*mmproj*.gguf` file auto-detected in the model's directory. Without one of these
tags, no injection occurs even if a sibling mmproj file is present. A model can
have both image-enabled and text-only profiles — the tag is the per-profile opt-in.
If the tag is set but no mmproj file is found, llml warns in the launch preview and
alert history and launches without multimodal support.

### `[profiles.hardware]` format

The `hardware` table maps to llml's local `hardware` object.

| Field         | Type    | Required | Description                                             |
| ------------- | ------- | -------- | ------------------------------------------------------- |
| `class`       | string  | no       | One of: `cpu`, `gpu`, `mixed`.                          |
| `gpu_count`   | integer | no       | Positive integer GPU count requirement.                 |
| `min_vram_gb` | integer | no       | Positive integer minimum VRAM requirement in GB.        |
| `max_vram_gb` | integer | no       | Positive integer maximum or tested VRAM envelope in GB. |
| `notes`       | string  | no       | Short hardware note or caveat.                          |

Normalization rules:

- Importers may normalize synonyms for `class`, matching local llml rules (for
  example `cpu-only -> cpu`, `hybrid -> mixed`).
- Blank or non-positive numeric values should be omitted.
- If both VRAM bounds are present and `min_vram_gb > max_vram_gb`, importers may
  swap them to preserve a valid range, matching local llml behavior.
- For `class = "cpu"`, importer normalization may clear GPU-specific numeric fields,
  matching local llml behavior.

### `args` format

Each element of `args` is one "panel row" - the same string a user would type into
llml's parameter panel. Two formats are accepted:

- `"--flag value"` - a flag and its value separated by a single space. llml splits
  this into two argv tokens at launch time.
- `"--flag"` - a standalone flag with no value.
- `"/path/to/something"` - a bare value with no leading dash.

Do not include the model path or the backend binary name; llml supplies those.

**Storage note:** The portable TOML format stores args as panel-row strings. When
import tooling writes to `model-params.json` it pre-splits each `"--flag value"`
into two separate tokens (`["--flag", "value"]`) to match llml's internal storage
format. This split is the importer's responsibility, not the LLM's - extract args
as panel-row strings in the portable TOML.

### `[[profiles.env]]` format

Each entry is a table with two string fields:

| Field   | Type   | Required | Description                 |
| ------- | ------ | -------- | --------------------------- |
| `key`   | string | yes      | Environment variable name.  |
| `value` | string | yes      | Environment variable value. |

## Example: llama.cpp profiles

```toml
schema_version = 3

[[profiles]]
name = "balanced-q4"
backend = "llama"
model_hint = "Llama-3-8B-GGUF"
args = ["--n-gpu-layers 80", "--ctx-size 4096", "--threads 8"]
use_case.primary = ["chat"]
use_case.tags = ["interactive", "balanced"]
hardware.class = "gpu"
hardware.gpu_count = 1
hardware.min_vram_gb = 24
hardware.max_vram_gb = 24
hardware.notes = "Tested on M1 Max 32GB unified memory."

[[profiles]]
name = "cpu-only"
backend = "llama"
model_hint = "Llama-3-8B-GGUF"
args = ["--n-gpu-layers 0", "--ctx-size 2048", "--threads 4"]
use_case.tags = ["low-memory"]
hardware.class = "cpu"

[[profiles]]
name = "max-context"
backend = "llama"
model_hint = "Llama-3-8B-GGUF"
args = ["--n-gpu-layers 80", "--ctx-size 32768"]
use_case.primary = ["chat"]
use_case.tags = ["long-context"]
hardware.class = "gpu"
hardware.min_vram_gb = 48

[[profiles.env]]
key = "LLAMA_CACHE_SIZE"
value = "8192"

[[profiles]]
name = "image"
backend = "llama"
model_hint = "gemma-4-26B-GGUF"
args = ["--n-gpu-layers 80", "--ctx-size 4096"]
use_case.primary = ["chat"]
use_case.tags = ["image", "interactive"]
hardware.class = "gpu"
hardware.min_vram_gb = 24

[[profiles]]
name = "text-only"
backend = "llama"
model_hint = "gemma-4-26B-GGUF"
args = ["--n-gpu-layers 80", "--ctx-size 8192"]
use_case.primary = ["chat"]
use_case.tags = ["interactive"]
hardware.class = "gpu"
hardware.min_vram_gb = 16
```

## Example: vLLM profiles

```toml
schema_version = 3

[[profiles]]
name = "single-gpu"
backend = "vllm"
model_hint = "Qwen2.5-72B-Instruct-AWQ"
args = ["--tensor-parallel-size 1", "--gpu-memory-utilization 0.95", "--max-model-len 8192"]
use_case.primary = ["chat"]
use_case.tags = ["interactive", "balanced"]
hardware.class = "gpu"
hardware.gpu_count = 1
hardware.min_vram_gb = 80
hardware.max_vram_gb = 80
hardware.notes = "Single A100 80GB, AWQ quantization."

[[profiles]]
name = "dual-gpu"
backend = "vllm"
model_hint = "Qwen2.5-72B-Instruct-AWQ"
args = ["--tensor-parallel-size 2", "--gpu-memory-utilization 0.90", "--max-model-len 32768"]
use_case.primary = ["eval"]
use_case.tags = ["throughput", "long-context"]
hardware.class = "gpu"
hardware.gpu_count = 2
hardware.min_vram_gb = 160
hardware.notes = "Two A100 80GB cards."
```

## Example: Ollama profiles

```toml
schema_version = 3

[[profiles]]
name = "gpu-full"
backend = "ollama"
model_hint = "llama3.2"
args = []
use_case.primary = ["chat"]
use_case.tags = ["interactive"]
hardware.class = "gpu"
hardware.notes = "All layers on GPU, 8k context."

[[profiles.env]]
key = "OLLAMA_NUM_GPU"
value = "999"

[[profiles.env]]
key = "OLLAMA_NUM_CTX"
value = "8192"
```

## Example: KoboldCpp profiles

```toml
schema_version = 3

[[profiles]]
name = "gpu-full"
backend = "koboldcpp"
model_hint = "Llama-3-8B-GGUF"
args = ["--gpulayers 80", "--contextsize 4096", "--threads 8", "--flashattention"]
use_case.primary = ["chat"]
use_case.tags = ["interactive", "balanced"]
hardware.class = "gpu"
hardware.gpu_count = 1
hardware.min_vram_gb = 24
hardware.notes = "CUDA GPU layers, 4k context."

[[profiles]]
name = "cpu-only"
backend = "koboldcpp"
model_hint = "Llama-3-8B-GGUF"
args = ["--gpulayers 0", "--contextsize 2048", "--threads 4"]
use_case.tags = ["low-memory"]
hardware.class = "cpu"

[[profiles]]
name = "vulkan"
backend = "koboldcpp"
model_hint = "Llama-3-8B-GGUF"
args = ["--gpulayers 80", "--contextsize 4096", "--usevulkan"]
use_case.primary = ["chat"]
use_case.tags = ["interactive"]
hardware.class = "gpu"
hardware.notes = "Vulkan backend for AMD GPUs."
```

## Compatibility

`schema_version = 3` is the current portable format. `schema_version = 2` is the
legacy format where `use_case.primary` was a single string (`primary = "chat"`)
instead of an array (`primary = ["chat"]`). Both `ReadPortable` and `FetchPortable`
accept v2 files and automatically migrate them to v3 in memory (single string becomes
a one-element array). New files should always emit `schema_version = 3`.

`schema_version = 1` was the earlier portable draft without structured `use_case` or
`hardware` metadata. It is not supported and will be rejected with an error.

## LLM extraction instructions

If you are an LLM reading this document in order to extract profiles from a source,
follow these rules:

1. **Backend detection:** Identify whether the source describes llama.cpp, vLLM,
   Ollama, or KoboldCpp invocations. Use the `backend` field accordingly (`llama`,
   `vllm`, `ollama`, `koboldcpp`). If the source covers multiple backends, emit one
   `[[profiles]]` entry per backend variant.

2. **Args extraction:** Extract only flags and values that appear explicitly in the
   source. Do not infer or invent values. Place each `--flag value` pair as a single
   string in the `args` array. Standalone flags (`--flash-attn`, `--no-mmap`) are
   single-element strings.

3. **Env extraction:** Extract environment variables set in the source (for example
   `export LLAMA_CACHE_SIZE=4096`, `OLLAMA_NUM_GPU=999 ollama run ...`). Place each
   as a `{key, value}` entry under `[[profiles.env]]`.

4. **model_hint:** Set to the model name or family mentioned in the source (for
   example the model card title or the model name in the `ollama run` command). Use
   the short name, not a full path.

5. **name:** Derive from the source context - for example `"default"`,
   `"4-bit-gpu"`, `"cpu-only"`, or the section heading the parameters appeared under.
   Keep it short.

6. **use_case:** When the source provides enough evidence, set `use_case.primary`
   as a TOML array using the canonical values (`chat`, `tool-calling`, `eval`). Add
   short lowercase tags only when the source materially supports them, such as
   `interactive`, `throughput`, `long-context`, or `low-latency`. If the source does
   not justify a field, omit it.

7. **hardware:** When the source provides enough evidence, set `hardware.class`,
   `hardware.gpu_count`, `hardware.min_vram_gb`, `hardware.max_vram_gb`, and
   `hardware.notes`. Put short compatibility notes or test context in `notes`. If
   the source does not justify a field, omit it.

8. **Do not extract model-location parameters into the portable profile.** Exclude
   flags and env vars that identify where to load the model from, because llml
   supplies the model path itself. Examples to exclude: `LLAMA_CACHE`, `-hf`,
   `--model`, `--lora`, `HF_HOME`, `--tokenizer`, and similar source-location or
   cache-location settings. For `koboldcpp`, exclude `--model`, `--lora`,
   `--mmproj`, `--tokenizer`, `--models-dir`, and any `-hf*` / `--hf-*` flags,
   as well as `LLAMA_CACHE`, `LLAMA_ARG_MODEL`, and `LLAMA_ARG_*` env vars
   that specify model file paths — the same model-location parameters that
   apply to `llama` also apply to `koboldcpp`.

   **`--mmproj` note (image/audio models):** `--mmproj` injection for `llama`
   and `koboldcpp` backends is **opt-in**: llml only injects a `--mmproj <path>`
   at launch when the active profile has `"image"` or `"audio"` in
   `use_case.tags`. When opted in, llml auto-detects a sibling `*mmproj*.gguf`
   file in the same directory as the model and injects the path. Portable profiles
   for image/audio-capable models (e.g. Gemma multimodal) therefore need not —
   and should not — include `--mmproj`; the path is machine-specific and llml
   supplies it automatically when the tag is set. If you need to override the
   detected path, add `--mmproj /explicit/path.gguf` to the profile args and llml
   will use your value instead (regardless of the tag).

9. **Output:** Emit valid TOML matching this schema. Set `schema_version = 3`. Use
   an array for `use_case.primary` (e.g. `primary = ["chat"]`). Do not include
   fields not listed in this spec. Do not include the model path or binary name in
   `args`.

## Versioning

`schema_version = 3` is the current portable format. Import tooling accepts v2 files
and migrates them automatically. Future versions will be documented here. Import
tooling should reject unrecognized schema versions with a clear error.

## Relation to internal model-params.json

The portable format and `model-params.json` are separate. The portable format is for
sharing — importing and exporting; `model-params.json` is the internal per-model
storage keyed by local model path. Import tooling bridges the two: it reads a
portable file, asks the user which local model to attach the profiles to, maps
portable metadata into the canonical local profile schema, and writes into
`model-params.json` under that model's key. Export tooling (the `E` TUI modal or
`llml export` CLI) reverses the direction — it reads `model-params.json` and
writes profiles into the portable TOML format defined here.
