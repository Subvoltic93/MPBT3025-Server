#!/usr/bin/env python3
"""Verify and print the initial House field in the preserved PlanetInfo cache."""

from __future__ import annotations

from collections import Counter
import hashlib
import json
import os
from pathlib import Path
import struct


ROOT = Path(__file__).resolve().parents[1]
CLIENT_ROOT = Path(os.environ.get("MPBT3025_CLIENT_ROOT", ROOT.parent.parent / "MPBT3025" / "client"))
VCR = CLIENT_ROOT / "MPBT3025andGILS/MPBT3025/BT3025.vcr"
SEED_DIR = Path(os.environ.get("MPBT3025_SEED_DIR", CLIENT_ROOT / ".runtime" / "server-seed"))
NAMES = SEED_DIR / "planet_names.json"
EXPECTED_SHA256 = "3c17cf8fee9c6bc028e193f455cefda70e00168a68ef92572bc5f27766ad007a"
HOUSE = {1: "davion", 2: "kurita", 3: "liao", 4: "marik", 5: "steiner", 7: "solaris"}


def main() -> None:
    data = VCR.read_bytes()
    if hashlib.sha256(data).hexdigest() != EXPECTED_SHA256:
        raise SystemExit("VCR hash differs from the preserved client cache")
    rows = json.loads(NAMES.read_text())
    if len(rows) != 121 or struct.unpack_from(">I", data, 0x3848)[0] != 121:
        raise SystemExit("Expected 121 planet records")
    cursor = 0x384C
    counts: Counter[str] = Counter()
    for expected_id, row in enumerate(rows, start=1):
        if row["id"] != expected_id:
            raise SystemExit(f"Unexpected planet ID {row['id']}")
        name = row["name"].encode("latin-1")
        prefix = struct.pack(">II", expected_id, len(name)) + name
        position = data.find(prefix, cursor)
        if position < 0:
            raise SystemExit(f"Planet {expected_id} was not found after {cursor:#x}")
        cursor = position + len(prefix)
        house_id = struct.unpack_from(">I", data, cursor + 1)[0]
        house = HOUSE.get(house_id)
        if house is None:
            raise SystemExit(f"Unknown House ID {house_id} for {row['name']}")
        expected_owner = row.get("initial_owner", "solaris")
        if house != expected_owner:
            raise SystemExit(f"{row['name']}: cache={house}, seed={expected_owner}")
        counts[house] += 1
        print(f"{expected_id:3}  {row['name']:<22} {house_id} {house}")
    print("Counts:", ", ".join(f"{name}={counts[name]}" for name in HOUSE.values()))


if __name__ == "__main__":
    main()
