# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`CONTEXT.md`** at the repo root: the domain glossary.
- **`dev-docs/llml/adr/`**: read ADRs that touch the area you're about to work in. The index is `dev-docs/llml/DECISIONS.md`.

`dev-docs` is a private git submodule (`flyingnobita/llml-internal`). If it is not checked out, or any of these files don't exist, **proceed silently**. Don't flag their absence; don't suggest creating them upfront. The `/mattpocock-domain-modeling` skill (reached via `/mattpocock-grill-with-docs` and `/mattpocock-improve-codebase-architecture`) creates them lazily when terms or decisions actually get resolved.

## File structure

Single-context repo:

```text
/
├── CONTEXT.md
├── dev-docs/                          ← private submodule
│   └── llml/
│       ├── DECISIONS.md               ← ADR index
│       ├── adr/
│       │   ├── 20260907-resolved-settings-instead-of-env.md
│       │   └── 20260907-split-discovery-cache-from-config.md
│       ├── YYYYMMDD-SPECS-short-title.md
│       └── YYYYMMDD-PLANS-short-title.md
├── cmd/
└── internal/
```

## Writing ADRs, specs, and plans

- **ADRs**: name them `dev-docs/llml/adr/YYYYMMDD-short-title.md` (dated, not numbered) and add a line to `dev-docs/llml/DECISIONS.md`.
- **Specs and plans**: keep the dated files in `dev-docs/llml/` (`YYYYMMDD-SPECS-*.md`, `YYYYMMDD-PLANS-*.md`) and index specs in `dev-docs/llml/SPECS.md`. When `/mattpocock-to-spec` publishes a spec as an issue, the issue links to the dated spec file rather than duplicating private detail (the issue tracker is public).

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `CONTEXT.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal: either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/mattpocock-domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR 20260907-resolved-settings-instead-of-env, but worth reopening because…_
