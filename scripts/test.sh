#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
overlay_file="$(mktemp "${TMPDIR:-/tmp}/mpbt3025-test-overlay.XXXXXX.json")"
trap 'rm -f "$overlay_file"' EXIT

python3 - "$root_dir" "$overlay_file" <<'PY'
import json
from pathlib import Path
import sys

root = Path(sys.argv[1]).resolve()
overlay_path = Path(sys.argv[2])
replacements = {}
tests_root = root / "tests"
for source in sorted(tests_root.rglob("*.go.txt")):
    relative = source.relative_to(tests_root)
    virtual = root / "src" / Path(str(relative)[:-4])
    if not virtual.parent.is_dir():
        raise SystemExit(f"test has no matching source package: {source}")
    replacements[str(virtual)] = str(source.resolve())
if not replacements:
    raise SystemExit(f"no test sources found under {tests_root}")
overlay_path.write_text(json.dumps({"Replace": replacements}, indent=2) + "\n")
PY

cd "$root_dir"
go_command="${GO:-go}"
"$go_command" test -overlay "$overlay_file" "$@" ./...
