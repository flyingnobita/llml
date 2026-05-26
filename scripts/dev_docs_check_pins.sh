#!/usr/bin/env bash
# Run from llm-launcher root. Delegates to dev-docs/scripts in the submodule.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT/dev-docs/scripts/check_parent_dev_docs_pins.sh"

if [[ ! -f "$SCRIPT" ]]; then
	echo "dev-docs-check-pins: missing $SCRIPT" >&2
	echo "Update the dev-docs submodule (needs llml-internal scripts/), then retry." >&2
	exit 1
fi

exec "$SCRIPT" \
	--catalog "${LLML_CATALOG_ROOT:-$ROOT/../llml-catalog}" \
	--launcher "$ROOT" \
	"$@"
