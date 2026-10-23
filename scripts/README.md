# Scripts

- `test.sh` runs the Go test suite by mapping test sources from `tests/` into their matching packages under `src/`.
- `native-combat-fixture.py` executes the supplied native module in an isolated i386 fixture process and prints JSON evidence to standard output. Save reviewed output under `data/test-fixtures/` when updating a fixture.
- `extract-vcr-planet-owners.py` checks the operator's local VCR and seed data.
- `summarize-trace.py` summarizes packet trace logs.

Client-dependent utilities read from the separate client directory and do not copy client assets into this project.

For targeted synthetic serializer fixtures (no live client injection), run:

```sh
python3 scripts/native-combat-fixture.py fire --weapon-slot 2 --target-entity 1
python3 scripts/native-combat-fixture.py server-fire --weapon-slot 2 --target-entity 1
```

`--target-entity -1` selects an untargeted shot. Overrides apply only to `fire` and `server-fire`; their default fixtures remain unchanged. These commands require the separately supplied native DLL and use the pinned build's serializer offsets.

For isolated damage-writer evidence, run `python3 scripts/native-combat-fixture.py damage` or `python3 scripts/native-combat-fixture.py local-damage`. These synthetic fixture commands execute the original writers and envelope round trip without launching Wine or contacting a server. They do not inject damage into a client.
