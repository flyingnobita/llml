# llml Domain Context

Shared vocabulary for the llml model launcher. These are project-specific concepts,
not general programming terms. Keep definitions tight; prefer the listed word over
its `_Avoid_` alternatives.

## Profiles

**Parameter Profile**:
A named set of extra env vars, argv, backend override, and use-case/hardware metadata
applied to one model at launch. Stored per model identity in `model-params.json`.
_Avoid_: config, preset, settings.

**Active Profile**:
The profile a model launches with — the one at `ActiveIndex`. In the `p` panel it is
also the profile currently being edited.
_Avoid_: current profile, selected profile.

**Working Form**:
The editing representation of a profile's env/args rows held in the panel buffer while
the `p` panel is open. May contain in-progress empty rows that the normalized storage
form never has. Args in working form are display-paired (`--ctx-size 4096` on one row);
storage form is flat argv.
_Avoid_: buffer state, draft, raw rows.

**Profile Editor**:
The module that owns the open panel's profile slice, the active index, and the
working-form env/args buffers. It materializes storage (flush working form into the
active profile, then normalize) only when read through `ActiveProfile()` / `Entry()`,
so callers never observe a stale active profile.
_Avoid_: param state, form model, panel controller.

**Materialize**:
Flush the working-form buffers into the active profile and normalize the result for
storage (stripping empty rows, expanding argv). Happens inside the Profile Editor's
read accessors, never as a separate step callers must remember.
_Avoid_: sync, commit, save (those mean other things here).

## Runtimes

**Model Format**:
The on-disk form of a model, which decides the Runtimes that can run it: GGUF
(llama.cpp, KoboldCpp), Safetensors (vLLM, oMLX, mlx-lm, mlx-vlm), NInfer (`.ninfer`),
Splash bundle, and Ollama library.
_Avoid_: model type, file type.

**Runtime**:
An inference server program llml launches or talks to for a model: llama.cpp,
KoboldCpp, vLLM, Ollama, NInfer, oMLX, Splash, mlx-lm, or mlx-vlm. A profile's
`backend` field names the Runtime it launches with; everywhere else, say Runtime.
_Avoid_: backend (outside the profile field), engine, server type.

**Unsupported Runtime**:
A Runtime that cannot run on the current platform (oMLX and Splash off Apple Silicon,
NInfer off Linux, mlx-lm and mlx-vlm on Windows and Intel Macs). It is absent from the
UI entirely.
_Avoid_: disabled runtime, unavailable runtime.

**Disabled Runtime**:
A supported Runtime the user has turned off. llml stops talking to it and will not
launch on it, but models that belong to it still appear, dimmed, so the user knows
they are there.
_Avoid_: hidden runtime, unsupported runtime, inactive runtime.

**Detected Runtime**:
A Runtime whose program llml can find, or whose server answers on its configured
address as that Runtime (another Runtime's server on a shared port does not
count). A Runtime seen for the first time starts on only if it is detected.
_Avoid_: installed runtime, available runtime.
