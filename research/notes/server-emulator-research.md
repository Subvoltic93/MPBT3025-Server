# BattleTech 3025 server emulator: initial investigation

Investigated October 19, 2026. This is an initial static reverse-engineering
assessment, not a recovered protocol specification or a working server.
Original client files were read without modification. No connection to the
historical EA endpoint was attempted. No new live packet trace was collected.

## Recommendation

Build a standalone **Go server**, with **Python research utilities** and
**SQLite persistence** initially. Keep the original client and isolate its
legacy wire protocol from accounts, lobbies, campaign rules, and combat.
Start as one executable with internal modules; separate processes only when
there is a demonstrated need. Target Linux, Windows, and macOS servers;
the existing Windows client remains a separate Wine/native compatibility issue.

The principal challenge is recovering behavior, packet layouts, and timing,
not raw throughput. Go gives us explicit binary encodings, ordinary TCP/UDP
sockets, a relatively small implementation surface, and portable builds.
The standard libraries document [networking](https://pkg.go.dev/net) and
[binary encoding](https://pkg.go.dev/encoding/binary); platform support is
listed in the [Go installation documentation](https://go.dev/doc/install/source).
The deployment build should avoid mandatory C dependencies; select the SQLite
driver with that requirement in mind. Test actual release builds on each OS.

C#/.NET is an equally credible choice if contributor familiarity favors it.
Python is excellent for probes and extraction; it could also handle an initial
server, but I favor Go's typed protocol code and deployment model for the
maintained implementation. Neither C++ nor Rust is necessary for speed at this
stage. Reconsider native code only for a measured bottleneck or exact behavior
that is impractical to reproduce otherwise.

### Contributor accessibility reassessment

The user's priorities are broad readability and ease of contribution. Go remains
my recommendation for balancing these with portable server deployment; this is
an engineering judgment, not a claim that Go has the largest contributor pool.
Python is likely the easier entry point for many occasional contributors, and
an established team with Python or C# expertise could reasonably choose either.
We currently have no contributor survey that establishes a language preference.

| Choice | Readability and workflow tradeoff for this project |
| --- | --- |
| Go | Relatively small language, explicit data types and errors, standard formatting and build/test tools. Pointers, interfaces and concurrency still need explanation; careless goroutine/channel use can make it hard to follow. Best overall fit in my judgment. |
| Python | Concise and approachable for research and game rules. Type hints and disciplined module boundaries can make a maintainable server. Interpreter/dependency packaging and asynchronous control flow need care; speed alone is no reason to reject it. |
| C#/.NET | Strong types and tooling, portable runtime, reasonable choice for a C#-experienced team. Keep the chosen language/framework surface small to support occasional contributors. |

Go's [style guide](https://go.dev/doc/effective_go) explains `gofmt`, and its
[project guide](https://go.dev/doc/code) describes the standard project workflow.
See also the [Python tutorial](https://docs.python.org/3/tutorial/) and
[.NET overview](https://learn.microsoft.com/en-us/dotnet/core/introduction).

Readability must be an implementation requirement:

* Prefer ordinary structs, functions, descriptive names, and explicit state
  transitions. Introduce interfaces or generics when a concrete need justifies
  them. Keep concurrent network handling outside sequential game-rule code.
* Keep packet byte handling in the protocol package. Game rules should read in
  terms of pilots, seats, purchases and battles, with named units and constants.
* Document each recovered packet with a byte diagram, provenance and example;
  explain uncertain historical behavior alongside the implementation.
* From the project root, use `go run ./src/cmd/server` to start the service and
  `./scripts/test.sh` to run tests. Keep first contributions free of Docker or
  a separate database service.
* Add a short contributor guide explaining package boundaries and a worked
  example of adding a message handler. Keep protocol documentation and editable
  game tables useful to contributors who do not write Go.
* Keep Python tools optional for normal server development; avoid splitting
  production game rules across languages.

If a usable 3025 implementation becomes available, evaluate its correctness,
license and maintainability before committing to a rewrite in Go. Reusing sound
existing work can outweigh the benefits of a preferred language.

## What the supplied files establish

| Evidence | Observation | Implication |
| --- | --- | --- |
| `BT3025.exe` PE header | Native 32-bit x86 Windows executable; linker timestamp September 24, 2001; symbols stripped | This needs native binary analysis, not .NET decompilation. A linker timestamp is not proof of release date. |
| Executable import table | Imports `AriesLib_InitializeConnection` from `aries.dll` | There is a narrow connection boundary to trace and instrument. |
| `aries.dll` export table | `AriesLib_GetVersionInfo` at RVA `0x2a5f`; `AriesLib_InitializeConnection` at RVA `0x29b2` | We have named entry points despite stripped application symbols. |
| Aries strings | `aol:`, `key:`, `WSOCK32.DLL`, `socket`, `connect`, `send`, `recv`, `ioctlsocket` | A legacy socket transport is recoverable. Complete framing, login semantics, and transport variants remain to be verified. |
| Executable strings | 118 distinct names ending in `_PDU` | Many lobby and game message families can be inventoried before decoding their layouts. These are names, not recovered numeric opcodes. |
| Protocol diagnostics | Labels for player IDs, rooms, profiles, planets, houses, weapons, armor, ranks, configuration sequence, and terrain seed | Diagnostic routines provide useful anchors for finding associated classes and serialization. Labels alone do not establish field order or width. |
| `BTCombat.dll` exports | `CRealityManager`, `CMechObject`, `AddHeat`, `ApplyDamage`, `ComputeCRC`, `SendShotCommand`, `ReceiveCombatData`, `StartLesson` | Substantial combat behavior exists locally. Whether server logic duplicates, validates, or merely relays each part is unresolved. |
| `mpbt.kdt` | Starts with `KSDT`; structured binary content | A data-container candidate requiring its own reader. Do not assume it is a complete server database. |
| `BT3025.vcr` | 61,107 bytes; first big-endian uint32 is 61,103, exactly the remaining size; readable pilot and mech names | Particularly useful serialized-data candidate. Its schema and exact purpose are not yet fully decoded. |
| Original manual | Defines pilot training, Solaris graduation, houses, rooms, purchases, progression, salary, and conquest | A functional acceptance checklist, subject to differences between manual and client versions. |

The original `Versions.TXT` explicitly identifies SurRender 1.42.2.9 and Kesmai
K3D components. We do not need to port the renderer to emulate the server.

### Bootstrap evidence

`GILS/gilstmp/gilsdata.gils` preserves a historical `STARTGAME` record, and
`GILS/gilslog.txt` records a successful run. The log shows a GET to
`www.ea.com:10080/EAPlay`, an `application/x-gils-plugin` response, and GILS
parsing a `STARTGAME` record for game ID 2088. Its connection component has this
shape, with the historical key omitted:

```text
cmdtxt=aol:<historical-server-address>:15000:key:<session-key>
```

The record also carries a separate member handle, numeric EA account ID,
session ID, instance ID, download parameters, and HTTP heartbeat fields. The log
shows GILS POSTing `/GameHeartbeat` every two minutes and sending `GAMEQUIT` at
exit. These HTTP heartbeats belong to the EA launch/service flow; they must not
be assumed to describe the Aries combat transport. The older failed
`gilslog2.txt` receives an HTML response containing “User ticket data not found”
and exits without parsing `STARTGAME`. Together with the manual's EA.com login
steps, this shows that GILS bridges an already-authenticated EA web session to
the game; it does not itself prompt for the member password.

Historical account, session, and ticket values are present in the raw files but
are unnecessary for implementation and must not be copied into notes. The logs
confirm the handoff shape, not the ticket-minting or validation rules. A local
emulator should continue to use a clearly local session key until those rules
are independently recovered; a guessed `user=...` launch argument leaves the
Aries account-name fields at `unknown`.

The log confirms the original launcher passed an equivalent connection string
to the game. Recreating the service still requires a local, well-defined account
and session handoff; these files do not specify EA's ticket-generation or game
server validation algorithm. Rebuilding the retired website and patcher is not
required for the first local playable milestone.

### Useful disassembly anchors

Addresses below are preferred virtual addresses for these specific binaries,
not universal offsets. Hashes are in `client/research/client-inventory.json`.

* `BT3025.exe:0x4dc637` and `0x4f1edf` call the Aries initialization import
  through IAT address `0x585998`; both pass `0x68` as the third argument.
* `aries.dll:0x100029b2` dispatches initialization by the third argument.
  The `0x68` branch constructs an object through `0x10001170`.
  This is a promising route to recovering the connection ABI and vtable.
* `BT3025.exe:0x52dd43` references the `STATIC_CONFIG_RESPONSE_PDU` diagnostic
  string; `0x52e2f1` references `STATIC_CONFIG_UPDATE_PDU`; `0x52ec91` references
  `BTCLIENT_ONLINE_PDU`. These are diagnostic anchors, not packet handlers
  identified solely by the string references.
* Candidate related serializer/deserializer routines appear around
  `0x52dce0`/`0x52dd00`, `0x52e1e0`–`0x52e2cd`, and
  `0x52ec20`–`0x52ec6b`. Follow class vtables and callers to confirm roles.
* `BT3025.exe:0x401d69` and `0x4023af` reference `BT3025.vcr` in file-stream
  paths. Nearby code invokes the same serialization-helper family found in
  protocol-related routines. This strengthens the cached serialized-data
  hypothesis, but does not prove the file is an on-wire packet capture.

Important named message families include:

```text
CLIENT_ONLINE_PDU / HOST_ONLINE_PDU / BTCLIENT_ONLINE_PDU
STATIC_CONFIG_UPDATE_PDU / STATIC_CONFIG_RESPONSE_PDU
CREATE_CHARACTER_PDU / MODIFY_CHARACTER_PDU / BUY_MECH_PDU
LIST_ROOMS_PDU / JOIN_ROOM_PDU / READ_PROFILE_PDU
JOIN_PLANET_PDU / REQUEST_PLANET_STATES_PDU
READY_ROOM_LIST_PDU / SET_SEAT_OPTIONS_PDU / COUNTDOWN_TIMER_PDU
CONFIGURE_COMBAT_PDU / START_COMBAT_PDU / ENTER_COMBAT_PDU
COMBAT_DATA_PDU / COMBAT_EVENT_PDU / COMBAT_SUMMARY_PDU
VICTORY_AWARDED_PDU / NOTIFY_COMSTAR_INTERDICTION_PDU
```

## Prior preservation work

**3025 has already had an independent partial emulator.** In his January 27,
2024 [demonstration description](https://www.youtube.com/watch?v=Yiv9tl4QD7Q),
Andrew Hassell describes recreating the server, says multiplayer still needs
substantial work, and says illness prevented him continuing. This is useful
first-party evidence of prior work, not proof that a complete playable server
or public source release exists. I did not locate a verified public 3025
source repository during this search. Obtaining source or protocol notes,
if the author chooses to share them, could save considerable duplicated work.

There is a public [Solaris emulator](https://github.com/kenhuman/mpbt-server),
written in TypeScript. Its README documents ARIES framing and partial game
restoration for **Solaris's `MPBTWIN.EXE`**, a different client. It describes
the older title's lobby server redirecting with ARIES type `0x03` and the client
opening a second TCP connection to a world server; that server then restores
room presence. Our supplied 3025 `aries.dll` has an outer type-3 connection
dispatch, making this a plausible architecture clue, but not proof that 3025
uses that exact handoff at `JOIN_PLANET`. Do not import its inner game-command
framing, numeric opcodes, or combat model into 3025 without checking our binary
and traces. The emulator is licensed AGPL-3.0; do not incorporate its code
without a separate license review.

## Architecture to aim for

```text
Original BT3025 client (Windows / Wine)
             |
       legacy Aries protocol
             |
     transport + framing + codecs
             |
        session / dispatcher
             |
 accounts -- pilots -- rooms/chat -- ready rooms -- combat
                    |                            |
             persistent campaign <--- match results
                    |
          SQLite + extracted game tables
```

Use explicit integer sizes, byte order, string encoding, and length limits.
Never serialize host-native structs or assume that a TCP read is a message.
Recover and implement fragmentation/reassembly if required: the client has
`SPLIT_BUFFER_PDU` and related diagnostics, but its exact rules are unknown.
Keep protocol version differences in codecs rather than scattered game rules.

Use serialized state transitions for each room/match and database transactions
for purchases, rewards, salary, and territorial updates. Session loss, ejection,
match abandonment, and reconnects need defined behavior. A match result must
not award money or conquest twice after a retry or restart.

Avoid choosing a combat tick rate or authoritative simulation model until the
client's packet cadence and ownership rules are established. A relay may be
enough for an initial experiment; it is not evidence of authentic combat or a
complete server. If authoritative simulation is required, port recovered rules
into the portable core. Do not make production depend on loading Windows
`BTCombat.dll` under Wine. A DLL-backed research harness can still serve as a
comparison oracle.

## Implementation sequence and acceptance gates

1. **Recover bootstrap and transport.** Import the EXE, Aries, and combat DLL
   into Ghidra; trace command-line parsing, socket initialization, and framing.
   Capture only a local controlled session with a listener and Wine socket
   logging/debugger instrumentation. Gate: a repeatable local connection with
   decoded initial traffic and no dependency on the old EA services.
2. **Decode configuration and enter the shell.** Trace the upload screen's
   actual state transitions, recover its expected acknowledgments and static
   tables, and decode the VCR candidate. Gate: the original client reaches
   pilot creation/lobby through genuine protocol exchanges. The screen text
   alone does not establish which request is blocking it.
3. **Restore the persistent pilot loop.** Pilot creation, Solaris status,
   house selection, profiles, chat, room rosters, mech ownership and purchases.
   Gate: two clients see each other and state survives a server restart.
4. **Restore one complete battle.** Ready seats, configuration, countdown,
   terrain/seed setup, movement, firing, damage, ejection, victory, return.
   Gate: two clients agree on a complete match, with results applied once.
5. **Expand to normal multiplayer and conquest.** Full lances, room lifecycle,
   planet availability/ownership, victory effects, ComStar interdiction,
   economy, rank/qualification and salary. Gate: an end-to-end campaign cycle
   works and persists correctly.
6. **Finish fidelity and operations.** Training lessons, practice modes,
   contacts/private messaging, statistics, house changes, failure paths,
   disconnects, restarts, and every other reachable feature supported by this
   build. Validate Windows/Linux/macOS server builds against the same fixtures.

Track every feature as observed, inferred, implemented, and verified. Define
"original functionality" against this preserved build first: prerelease
promises and features from other BattleTech titles are not equivalent evidence.
The manual should become a feature matrix with page references and live-client
acceptance checks. Never mark restoration complete merely because menus open.

Protocol tests should use captured or client-generated byte fixtures, including
partial/coalesced TCP reads, invalid lengths, sequence handling and disconnects.
Combat tests should compare against recovered behavior and multi-client traces,
especially numeric rounding, timing, random seeds and agreement on damage.
An encode/decode round trip alone can reproduce the same wrong assumption twice.

## Immediate next work

The highest-value next implementation is a localhost bootstrap/trace harness
and a documented first handshake, followed by the smallest configuration
exchange that gets past the current startup boundary. Defer broad gameplay
implementation until that exchange is verified. Full restoration looks
plausible, but neither completeness nor a reliable schedule can be established
before transport and combat responsibilities are recovered.

## Reproducing this initial inspection

From the repository root, run `python3 client/scripts/inspect-client.py` for hashes,
PE sections, PDU names and file/virtual offsets. It uses only Python's standard
library and reads originals.
The generated inventory is saved at `client/research/client-inventory.json`.
Local `client/research/raw/` contains `strings -a -t x`, `objdump -p`, executable
disassembly and manual text used during this pass. These are generated working
materials, not recovered source code. Keep original assets and private session
records out of any future public emulator release.
