# MPBT3025 Server Emulator

> **Live project status site:** [MPBT 3025 restoration status](https://subvoltic93.github.io/MPBT3025-Server/)

A Go implementation of the local server side of the MPBT 3025 protocol. The project focuses on connection bootstrap, pilot and campaign state, Solaris rooms, and the current multiplayer combat path. It does not contain the original client, client binaries, art, maps, or the reference project's source.

The organization takes inspiration from the separation of source, research, and utilities in [KenzHuman's `mpbt-server`](https://github.com/kenhuman/mpbt-server). This implementation remains Go and keeps its protocol and state packages small and explicit.

## Current state

The server can bootstrap the client, persist local pilot/world state as JSON, publish planet and Solaris room state, route live occupants and chat, coordinate mission seats and countdown, and relay observed combat position/fire commands. Combat fire relay accepts standalone and batched shots, checks the stock weapon-slot range, launched-room collision entity and Mech armor-part range, and authenticates the shooter. An opt-in armor diagnostic now shows one non-lethal laser armor reduction per target consistently in both clients. Authoritative aim/hit validation, full damage rules, match completion, and results are still unfinished. The October 23 trial reconfirmed same-city Solaris counts, Alpha/Beta seats, Back/rejoin, chat filtering, ready/cancel behavior, and two-client native combat entry. Solaris staging rosters now exclude other cities; that isolation has a regression test, while a native trial with pilots in separate cities remains pending.

See [Research.md](Research.md) for evidence and open questions, [Roadmap.md](Roadmap.md) for planned work, and [Protocol.md](Protocol.md) for listener addresses, framing, handled messages, and the current connection flow. The [October 23 session checkpoint](research/notes/solaris-session-2026-10-23.md) records the moved workspace, current trial, and verification limits. The [implementation handoff notes](research/notes/bootstrap-handoff.md) record detailed development history. The game manual and SDS are indexed in [`reference/README.md`](reference/README.md).

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

## Connect a local client

The client receives the server endpoint at launch through its legacy `aol:` argument; it is not entered in an in-game server-address screen. The argument format is `aol:<server-host>:<server-port>:key:<local-key>`. The launcher and game files live in a separate local client workspace; this server repository does not include or distribute them.

### Windows

In PowerShell, from the root of this server repository, start the Go server:

```powershell
go run ./src/cmd/server -listen 127.0.0.1:15000
```

In a second terminal, launch the existing Windows client from the directory containing `BT3025.exe`. Replace the example path with the location of your own client installation:

```cmd
cd /d "C:\path\to\MPBT3025andGILS\MPBT3025"
BT3025.exe "aol:127.0.0.1:15000:key:local-dev"
```

The `local-dev` value is a development key, not an EA account or password. This direct launch bypasses the retired EA/GILS sign-in and launch service.

### Linux with Wine

In one terminal, from this repository, start the server:

```sh
go run ./src/cmd/server -listen 127.0.0.1:15000
```

In another terminal, from the workspace root that contains your separate `client/` directory run the existing helper:

```sh
./client/scripts/launch.sh game aol:127.0.0.1:15000:key:local-dev
```

### Client on another computer on your LAN

On the server computer, start the server from this repository bound to its network interfaces:

```sh
go run ./src/cmd/server -listen 0.0.0.0:15000
```

On the Windows client, use the server computer's reachable LAN IPv4 address in the `aol:` argument. For example, if `ipconfig` shows `192.168.1.25` on the server, run this from the directory containing `BT3025.exe`:

```cmd
BT3025.exe "aol:192.168.1.25:15000:key:local-dev"
```

For a Linux/Wine client, pass that same argument to `./client/scripts/launch.sh game`. Allow inbound TCP port `15000` through the server computer's firewall for the trusted LAN. Do not put `0.0.0.0` in the client argument; it is only the server's bind address. Keep this unauthenticated prototype on a local or trusted network.

The optional secondary listener on port `15001` is experimental and only used with `-planet-redirect`; it is not needed for the tested direct-launch path.

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

## Experimental spawn arrangement

Native `FillMech` scales setup coordinates by 0.01 and computes facing toward the map origin (`0x6904207e`–`0x690420fd`). The experimental launcher now mirrors opposing lances across that origin: first seats are 200 metres apart, with 30 metre seat spacing. This controlled diagnostic arrangement replaces the almost parallel, 20 metre separation used previously. The October 23 follow-up confirmed mutual sight, 200 metre target selection, targeted laser reports in both directions and the incoming laser effect. The later opt-in armor diagnostic verified one synchronized non-lethal armor change per target. These remain diagnostic spawns, not recovered production spawn rules.

Native damage research now establishes absolute remaining armor assignments and overflow into internal structure using isolated DLL execution. The protocol encoder is covered by native writer fixtures; the opt-in diagnostic now emits one armor assignment per target, with synchronized native HUD changes verified. The server also caches the latest reported X/Y and 15 opaque pose-field bytes for each authenticated mission entity to support pose and HitReturn correlation; these client values do not authorize damage. See the [combat notes](research/notes/combat-command-stream.md).

## Controlled armor diagnostic

`-experimental-combat-armor-probe` requires `-experimental-combat-init` and is disabled by default. It trusts a valid room-scoped medium-laser collision report and applies one non-lethal 5-point stock armor reduction per target. Both clients receive the same absolute assignment followed by fire, updating the own-Mech and target damage panels together. The October 23 trial verified both directions and confirmed later shots add no further diagnostic damage. This is a receive/routing check; collision geometry, repeated damage, internal/critical damage, destruction and match results still need server rules.

## Experimental cumulative laser damage

Add `-experimental-combat-damage` together with `-experimental-combat-init` for repeated stock COM-4D medium-laser damage. It cannot be combined with `-experimental-combat-armor-probe`. The server owns armor/internal totals and recharge, requires recent reported poses, checks the native 270m range and a coarse target envelope, and sends identical assignment/fire streams to both clients. Native fixture and server tests cover depletion, internal transfer, weapon disable and core/head destruction state. Live cumulative damage delivery is pending at the implementation checkpoint.

This mode still trusts client movement and the collision part within an experimental 10m horizontal / -1..20m height envelope. Aim, occlusion, heat, criticals, piloting-target changes, ammunition weapons and match results remain unfinished. See the latest [damage checkpoint](research/notes/solaris-session-2026-10-23.md).
