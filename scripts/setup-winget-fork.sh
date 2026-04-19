#!/usr/bin/env bash
# Ensure a winget-pkgs fork exists under your GitHub user and sync it with microsoft/winget-pkgs.
# Requires: gh CLI, authenticated via `gh auth login` (scopes: repo, read:org).
# Optional: WINGET_FORK_OWNER=yourname if the fork should not be the logged-in user (rare).

set -euo pipefail

upstream="microsoft/winget-pkgs"
fork_owner="${WINGET_FORK_OWNER:-$(gh api user -q .login)}"
fork_repo="${fork_owner}/winget-pkgs"

if [[ "${fork_owner}" != "flyingnobita" ]]; then
	echo "warn: GoReleaser in .goreleaser.yaml expects owner flyingnobita/winget-pkgs; your fork is ${fork_repo} — update winget.repository.owner if needed." >&2
fi

if ! gh repo view "${fork_repo}" >/dev/null 2>&1; then
	echo "Fork not found; creating ${fork_repo} from ${upstream}..."
	gh repo fork "${upstream}" --clone=false
else
	echo "Fork ${fork_repo} already exists."
fi

echo "Syncing ${fork_repo} default branch with upstream (branch: master)..."
gh repo sync "${fork_repo}" -b master

echo "Done. Add repo secret WINGET_GITHUB_TOKEN on flyingnobita/llml (PAT with push to ${fork_repo})."
