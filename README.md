# MPBT3025 Server Emulator

A Go implementation of the local server side of the MPBT 3025 protocol. The project focuses on connection bootstrap, pilot and campaign state, Solaris rooms, and the current multiplayer combat path. It does not contain the original client, client binaries, art, maps, or the reference project's source.

The organization takes inspiration from the separation of source, research, and utilities in [KenzHuman's `mpbt-server`](https://github.com/kenhuman/mpbt-server). This implementation remains Go and keeps its protocol and state packages small and explicit.

## Current state

The server can bootstrap the client, persist local pilot/world state as JSON, publish planet and Solaris room state, route live occupants and chat, coordinate mission seats and countdown, and relay observed combat position/fire commands. Authoritative damage, match completion, and results are still unfinished. The Solaris room count and Alpha/Beta display mapping have a reported regression and remain under verification.

See [Research.md](Research.md) for evidence and open questions, [Roadmap.md](Roadmap.md) for planned work, and [Protocol.md](Protocol.md) for listener addresses, framing, handled messages, and the current connection flow. The [implementation handoff notes](research/notes/bootstrap-handoff.md) record detailed development history. The game manual and SDS are indexed in [`reference/README.md`](reference/README.md).

## Run

Requires Go 1.24 or newer. From this directory:

```sh
go run ./src/cmd/server
```

The default listener is `127.0.0.1:15000`. The server writes campaign state to `runtime/world.json` and packet traces to `runtime/traces/`. Both paths can be changed with `-world` and `-trace-dir`.

For example, an isolated local trial can use:

```sh
go run ./src/cmd/server \
  -listen 127.0.0.1:15011 \
  -world runtime/world-test.json \
  -trace-dir runtime/traces/test
```

Run `go run ./src/cmd/server -h` for the available flags. Planet-state probes, room-type trials, combat initialization, and the second listener are experimental; use them only with an isolated local client.

## Layout

```text
.
├── data/                 Go seed loader and generated protocol fixtures
│   └── test-fixtures/    native combat command JSON fixtures
├── reference/            manual and SDS reference copies and index
├── research/notes/       dated implementation and investigation notes
├── scripts/              trace, fixture, test, and site utilities
├── src/
│   ├── cmd/server/        executable entry point
│   ├── config/            command-line configuration and validation
│   ├── db/                reserved database adapter and schema placeholders
│   ├── protocol/
│   │   ├── aries/         outer TCP framing and redirect messages
│   │   └── game/          inner game PDU and combat codecs
│   ├── server/            listener, connection flow, sessions and room fanout
│   └── state/             campaign rules and JSON persistence
├── tests/                 tests grouped by source package; files use .go.txt
├── README.md, Protocol.md, Research.md, Roadmap.md
├── index.html             GitHub Pages status homepage
├── project-files.html     links to documentation, research and downloads
├── assets/site.css        shared retro layout and responsive styles
└── runtime/               ignored local state, traces, logs and binaries
```

All test sources live under `tests/`. The files use a `.go.txt` suffix so Go does not treat the test directory as a second copy of each package. `scripts/test.sh` maps them into their matching source packages with a Go overlay, preserving tests of package-private behavior.

The campaign store is currently JSON-backed. `src/db/` is a documented placeholder for a future persistence adapter; it has no database driver, actual SQL tables, or runtime role yet. `state` owns current persistence and game rules; `protocol` owns wire encoding; `server` coordinates sessions and request flow.

The default seed uses generic planet labels and a synthetic ownership map. To use operator-supplied seed tables, place `planet_names.json` and optionally `planet_sds_owners_v1.json` in an external directory and set `MPBT3025_SEED_DIR`. Scripts that inspect a supplied client cache or native module look outside this project; set `MPBT3025_CLIENT_ROOT` to that separate client directory. Native protocol fixtures under `data/test-fixtures/` are generated test artifacts, not extracted campaign seed tables.

## Build and checks

```sh
go build ./...
./scripts/test.sh
```

The status page is the GitHub Pages homepage at [`index.html`](index.html). The former [`project-status.html`](project-status.html) path forwards to it. Site documents are built from the Markdown sources with:

```sh
python3 -m pip install -r requirements-site.txt
python3 scripts/build_site.py
```

The generator writes static HTML next to the source tree; GitHub Pages serves the committed files directly from the repository root. See [`project-files.html`](project-files.html) for the site index and downloads.
