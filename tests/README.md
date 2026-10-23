# Tests

Test sources are grouped here by their corresponding package under `src/`. Their `.go.txt` suffix keeps Go from discovering these directories as duplicate packages while leaving the tests physically separate from production code.

Run the suite from the project root with `./scripts/test.sh`. The script uses a temporary Go overlay to expose each test source at the matching `src/` package path, so tests can still inspect package-private protocol and state behavior. Combat command fixture JSON is stored in [`../data/test-fixtures/`](../data/test-fixtures/).

The Solaris presence regression checks city-scoped roster/count snapshots and arrival/disconnect isolation. The planet ownership codec test sets representative owners explicitly, so it does not assume a private client seed or the optional operator seed directory. Native UI evidence and remaining gaps are recorded in the [October 23 checkpoint](../research/notes/solaris-session-2026-10-23.md).
