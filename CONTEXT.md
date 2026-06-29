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
