#!/usr/bin/env bash
# Build this checkout and install it as llml-dev in ~/.local/bin.
#
# The -dev name keeps a source build from shadowing a released llml (Homebrew,
# or scripts/install.sh, which also uses ~/.local/bin), so `llml` always means
# the installed release and `llml-dev` this checkout.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin_dir="$HOME/.local/bin"
target="$bin_dir/llml-dev"

mkdir -p "$bin_dir"
# No version stamp: an unstamped build reports "dev (<commit>, <date>)", which is
# what tells it apart from a release.
(cd "$repo_root" && go build -o "$target" ./cmd/llml)

echo "Installed $target"
echo "llml-dev --version: $("$target" --version)"

case ":$PATH:" in
*":$bin_dir:"*) ;;
*) echo "Note: $bin_dir is not on your PATH." >&2 ;;
esac
