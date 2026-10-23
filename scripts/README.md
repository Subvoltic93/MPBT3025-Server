# Scripts

- `test.sh` runs the Go test suite by mapping test sources from `tests/` into their matching packages under `src/`.
- `native-combat-fixture.py` executes the supplied native module in an isolated i386 fixture process and prints JSON evidence to standard output. Save reviewed output under `data/test-fixtures/` when updating a fixture.
- `extract-vcr-planet-owners.py` checks the operator's local VCR and seed data.
- `summarize-trace.py` summarizes packet trace logs.

Client-dependent utilities read from the separate client directory and do not copy client assets into this project.
