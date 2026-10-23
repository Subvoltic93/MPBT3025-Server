# Local bootstrap implementation handoff

Historical local paths in this document are relative to the original workspace root. Original client assets, client-derived research, and visual evidence remain in the separate workspace-root `client/` directory and are not included in this server project.

Work resumed on 2026-10-19. Original client assets are untouched; experiments use the project Wine copy.

## Current state

**Resume here:** [Solaris session handoff, 2026-10-21](solaris-session-2026-10-21.md)
contains the latest live-session checkpoint and historical client evidence.
The current project layout and active development notes are in the root README
and project status page.

**Latest correction (2026-10-21, resumed session):** `PlayerID` is a string,
not a numeric uint32. The prior nonzero-ID trials were malformed string
encodings; they do not establish an EA authentication requirement. Correct
string IDs and removal of peer `0x3c` updates produced a live map roster with
both BetaPilot and Subvoltic (count 2). Screenshot:
`client/.runtime/captures/string-id-b-two-players.png`; trace:
`runtime/traces/string-player-id/session-1792608043213374200.log`.
The recovered `0x3e UPDATE_ROOM` with lobby type 1 also moved the live client
into **Staging Area / Missions on Solaris**, with **You are on Solaris** in
Communications. Capture: `client/.runtime/captures/room-update-type1-a-live.png`.
See the latest recovery section at the end for addresses and validation.
Older empty-ID and unknown-transition conclusions below are superseded.

This handoff preserves dated implementation history. Source paths and
embedded seed claims in older entries describe that checkpoint; the
current source uses `src/state`, the optional seed loader is in `data/`,
and test sources and protocol fixtures are grouped under `tests/` and
`data/test-fixtures/`. The current manual and SDS copies are under
`reference/`; client assets, extracted cache data, and extracted SDS text
remain outside the project.

The broader gameplay state model and its evidence are summarized in
[`world-state-research.md`](world-state-research.md). That research confirms
that the client's map, sector selection, staging rooms, mission seats, pilot
progression, and planetary control all depend on shared server state.

## Prototype game-state implementation

`src/state` now separates pilot/campaign rules from wire messages.
It persists a world JSON file containing 121 planets, five Houses, pilots,
Solaris's 16 city lobbies, strategic sectors, mission rooms, eight-seat lances,
ready countdowns, mech ownership, C-bills, salary claims, and idempotent match
results. Cadets start on Solaris with a Commando-4D; completed Solaris matches
award prototype rank points, and 1,200 points unlock House selection. Campaign
battle results can shift planet control. This is the foundation for all game
surfaces described in the manual, not yet a claim that their wire handlers or
the combat simulation are complete.

The planet names come from the preserved client cache. Ownership is split
evenly among Houses and neighbor links form a ring only to give the prototype
map and eligibility rules usable defaults. Sector names, room templates,
scoring, mech prices, and salary multipliers are also provisional. These
values can be edited in `runtime/world.json` and will be reset when that file
is deleted. Research on the separate MPBT: Solaris emulator supports the
service boundaries (accounts, lobby/world travel, rooms, combat); its
`MPBTWIN.EXE` inner protocol is unrelated to BT3025's PDU IDs.

The Go server completes Aries login and sends a correctly framed HOST_ONLINE greeting. In an earlier run, **the client reached the Inner Sphere map shell, loaded Basic Training, and returned to the map** after an experimental zero-result ID0x88 reply to its ID0x87 create-character request. This confirms that reply was sufficient to reach the map shell in that run; it does not establish that the server persisted a character or that the reply is the authentic success exchange. The map shell is not the same screen as a planet's staging-area lobby.

The earlier failed join experiment is `runtime/traces/session-1792434813246446403.log`. The client replayed its saved `Subvoltic` create-character request, then sent ID0x91 for Solaris (planet 121). The server answered with ID0x90 and eight zero bytes; the client then displayed a Microsoft Visual C++ Runtime Error (“abnormal program termination”). A later vtable and diagnostic-string inspection corrected the old opcode mapping: ID0x90 is `COMBAT_SUMMARY_PDU`, ID0x91 is `JOIN_PLANET_PDU`, and ID0x92 is `JOIN_PLANET_FAILED_PDU`. The crash followed a malformed combat-summary body, not a valid join-failure response. We have not tested the correctly typed ID0x92 message. In the subsequent pre-change run, `runtime/traces/session-1792434956177851188.log`, the client replayed the saved call sign and Solaris join automatically. The server ignored ID0x91 and the client stayed on the map without crashing. It also sent ID0xa7 planet-state requests, which the server ignored. The earlier map capture at `client/.runtime/client.png` showed black faction regions after the empty ID0xa8 update. The user reports the top controls were gray and unavailable.

The previous full-bootstrap trace is `runtime/traces/session-1792431406491653529.log` (latest activity 10:48 local). Player ID0x3c has a 43-byte body. Profile ID0x3d now has a 101-byte body (107 bytes including its PDU header), and the client consumed it without a length error. The first experimental profile body had omitted the `uint32` at profile offset +0x94, which the serializer emits after +0xbc; adding it fixed the packet length. The same trace shows the client's 0xa4 sequence-0 acknowledgement, the synchronized 0x86 update, and a later heartbeat.

WineDbg confirmed the inbound 0x3c PDU wraps a `BTPlayerInfo` object and that its body passes the exact-length check. The 0x3d PDU wraps profile data; after the +0x94 field was added, the next decoder breakpoint was reached without hitting the mismatch branch at 0x511a43. Earlier failed profile traces (`session-1792430467711164407.log` and `session-1792430567672060019.log`) are retained as history. A player-only run (`session-1792430969172627005.log`) stayed on the upload screen, confirming that profile data was also required.

The call-sign form's OK button sends ID0x87 with body `00000009 537562766f6c746963 00000000` (length-prefixed `Subvoltic`, followed by an empty counted list). The client serializer at `BT3025.exe:0x52d720` identifies ID0x87 as `CREATE_CHARACTER_PDU`; its fields match the trace. Before the server handled this request, it logged and ignored ID0x87, leaving the form open while the client retried.

The client has an explicit ID0x88 receive path at `BT3025.exe:0x40d338`. It branches on the first result field: zero clears the pending create-character form state, while nonzero enters an error-display path. The `CREATE_CHARACTER_FAILED_PDU` base serializer at `0x511e10` writes that uint32 result followed by a length-prefixed string. The Go server decodes ID0x87 and sends ID0x88 with zero result and an empty string (eight zero body bytes). In the latest run, this cleared the call-sign form and the client reached the Inner Sphere map shell. Treat this as a working bootstrap experiment, not proof of the authentic success message or durable character creation.

For that experiment, the client and server were restarted. The session trace is `runtime/traces/session-1792432410938502234.log`; login, player/profile updates, settings, room list, and config acknowledgement replayed. It records the ID0x87 call-sign request, the server's ID0x88 zero-result reply, and then ID0xa7 planet-state requests. At map-shell entry, the client showed the Inner Sphere map and Training panel. The user completed Basic Training and returned to the map shell; the earlier capture shows Advanced Piloting selected.

The client sends ID0xa7 with a four-byte zero body alongside empty ID0xa3 heartbeats. Corrected vtable/class-prefix inspection identifies ID0xa7 as `REQUEST_PLANET_STATES_PDU`, not `REQUEST_LOBBY_ROOMS_PDU`: its serializer writes one uint32 planet ID, and zero is the observed request value. The neighboring ID0xa8 class is `PLANET_STATE_LIST_UPDATE_PDU`; its serializer at `BT3025.exe:0x5239d0` writes a counted container, a uint32 sequence number, and a uint16 seconds-until-shift. Its decoder is at `0x523a10`; `0x523a50` is the diagnostic field formatter. In the 11:29 experiment, the Go server returned an empty 10-byte ID0xa8 body. The client accepted the packet, but the map's faction-colored territories rendered black afterward. Treat the all-zero update as an incomplete placeholder that changes the map display; do not mistake it for valid planet-state data.

### Latest VCR-state experiment (12:23 local)

The trace `runtime/traces/session-1792437754057918789.log` shows the client sending ID0x87 for `Subvoltic`, receiving the experimental zero-result ID0x88 completion, and then requesting planet state with ID0xa7 for ID 0. With `-planet-vcr` enabled, the server sent the 28,114-byte VCR-derived ID0xa8 candidate in two ID0x47 split-buffer frames (16,384 and 11,752 bytes). Immediately afterward the client displayed a Microsoft Visual C++ Runtime Library error: “abnormal program termination.” There was no later client packet in that trace, and the crash happened before the user clicked **Go To Planet**.

This ties the failed run to the new ID0xa8 delivery, but does not distinguish a split-buffer receive/reassembly problem from an invalid ID0xa8 record layout or values. The VCR table and network state-list element type are still unproven matches. Keep `-planet-vcr` disabled for normal client runs. The clean restart trace is `runtime/traces/session-1792437926599845347.log`; it contains the saved ID0x87 call-sign request for `Subvoltic`, then ID0x91 for Solaris and repeated ID0xa7 requests, with no ID0xa8 response. The screenshot taken before the user repeated the progression showed the map under a blank call-sign pane. The client stayed open without the large update.

### Single-record planet-state trial (14:06 local)

The server restarted with `-planet-vcr-solaris`, which sends one 202-byte ID0xa8 PDU containing count 1, the 186-byte Solaris VCR record, sequence 0, and timer 0. Trace `session-1792444010046251716.log` shows the client replaying `Subvoltic` creation, receiving the zero-result ID0x88 completion, then receiving session-specific ID0x3c/0x3d pilot/profile records. It requested planet state with ID0xa7 and received the compact ID0xa8 candidate. The client process was still alive when the trace was first reviewed, but the user later reported that the menus became visually corrupted and the game crashed after advancing from the call-sign screen. The run therefore failed; it does not establish that the compact record was accepted.

There are two new variables in that run: the session-specific ID0x3c/0x3d records and the compact ID0xa8 record. Neither has been isolated as the crash cause. The next run removes both from the live path: the server no longer emits duplicate session-specific pilot/profile records after ID0x88, and it is started without either planet-state experiment flag. The persistence model and stored call sign remain enabled. Once this baseline is stable, reintroduce one message at a time.

### Current planet-entry attempt (12:25–12:38 local)

The user entered `Subvoltic`, completed Basic Training, exited training, selected Solaris, and clicked **Go To Planet**. In the current trace, the server sends the experimental zero-result ID0x88 create-character completion; the client then sends ID0x91 with body `00000079 00000000` (Solaris ID 121 and an empty string). The server logs the join request but has no validated success reply. The client continues polling with ID0xa7 for planet ID 0 and sending ID0xa3 heartbeats. No ID0xad lobby-room request or ID0xa8 planet-state update appears.

The screenshot captured at 12:29:45 (`client/.runtime/client.png`) still has the **Inner Sphere Map: 3025 AD** title and the selected Solaris card. **Go To Planet** remains active; the lower **Communications** and **MechWarriors on Planet** panels are empty. The user reports this is the only available server-related path. The Player's Guide says **Go To Planet** should show strategic sectors and their 'Mech occupancy before the pilot can choose **Go To Sector**. The screen therefore has not visibly reached the sector list or staging-area lobby. We cannot infer from the map screenshot whether the client applied any part of the join request; the trace only establishes that it sent one and received no join reply.

This is a stronger current blocker than the earlier blank-call-sign screenshot: the client now has a call sign, has completed Basic Training, and can issue the Solaris join request, but the server still lacks a proven join transition and valid planet/sector state. Do not send the experimental ID0xa8 VCR candidate again; its previous live delivery terminated the client, and its record layout remains unknown. Continue by decoding the A8 element type and identifying the server message that populates the strategic-sector view before another live trial.

### Clean baseline after the reported crash (14:10 local)

The user reported that the previous run's menus became distorted and the client crashed after advancing from call-sign creation. The previous trace contains both additional session-specific ID0x3c/0x3d records and the compact experimental ID0xa8 update, so it does not identify which caused the failure. I removed the post-creation 0x3c/0x3d sends and restarted without either VCR planet-state flag.

In the resulting trace, `session-1792444272200756933.log`, the client completed the same call-sign flow, sent its ID0x91 Solaris join request, and continued polling with ID0xa7. The server sent neither a duplicate pilot/profile record nor ID0xa8. The user clicked **Go To Planet**; the screen stayed on the Inner Sphere map with Solaris selected and the lower planet panels empty. The client remained stable, but did not enter the strategic-sector list. This shows the crash does not reproduce in the no-experiment baseline. It still does not distinguish the two experimental messages as the cause, and the join remains unhandled at the UI level.

### Isolated Solaris-state trial (14:17 local)

The next trial enabled only `-planet-vcr-solaris`; it did not send the later
session-specific ID0x3c/0x3d updates. Trace
`session-1792444595134542990.log` shows startup, an ID0xa7 request for planet
0, and the server's 202-byte ID0xa8 candidate. No later client PDU appears.
The user saw the Microsoft Visual C++ Runtime Library “abnormal program
termination” dialog while the map and blank call-sign pane were visible. The
client run therefore fails with the single-record update by itself, before
call-sign creation or the ID0x91 join request.

The VCR bytes explain why this candidate is invalid as a direct network record.
The Solaris cache entry starts with ID 121, length-prefixed `Solaris`, and type
byte 2; the remaining string lengths (`7` and `133`) are stored together before
the strings. The `PlanetInfo` network serializer at `BT3025.exe:0x515ca0`
interleaves each string length with its text. In the candidate record, the
network decoder would read `00 00 00 85 53 6f 6c` as the first post-type string
instead of `Solaris`. The ID0xa8 element type remains unknown, but this rules
out sending the VCR record bytes unchanged. Both `-planet-vcr` flags stay off.

The Player's Guide v1.4.5 page 8 says a pilot who completes Pilot Training and
clicks **Go To Planet** appears in a Solaris city lobby. Pages 13–15 describe
the later graduated-pilot flow through planet sectors and their staging areas.
The user's current pilot is still a cadet, so the immediate acceptance target
is the Solaris city lobby, not House selection or the Inner Sphere sector
list.

The next no-A8 client run restores the real call sign to the client after
creation: the server will resend ID0x3c/0x3d updates using the persisted call
sign instead of leaving the startup `Local Pilot` placeholders in place. This
trial will also determine whether those profile refreshes are safe without the
bad ID0xa8 update.

### World-state and disabled-control findings

The user's reading is directionally right: normal Inner Sphere play needs shared faction and territory state, not only a lobby handshake. The manual describes five Houses contesting 120 territories. Each planet has an owner plus five House control percentages; ownership and adjacency determine which planets can be attacked or defended, and battle results shift those percentages. The client map expects these live values: a structurally valid but empty ID0xa8 update turned its territory fills black.

There are two separate issues. First, the previous server ignored `JOIN_PLANET` (ID0x91). The malformed ID0x90 response was `COMBAT_SUMMARY`, not the typed failure message. A later experiment sent an empty ID0x97 ready-room list after ID0x91; the client stayed on the map and did not request lobby rooms (ID0xad), so the server no longer sends that unsupported early reply. The ID0x97 list remains available only in response to ID0xad. Second, the original local pilot/profile were placeholders with no House or earned rank. The new game package persists the call sign and campaign state, but still sends a placeholder at initial login until the client supplies its character record. The manual says new pilots train and fight on Solaris, need 1,200 rank points for the Graduation Medal, and only then choose a House; it explicitly says Find Planet is unavailable before graduation and that some new-pilot actions are grayed out. Basic Training completion is not the same as Solaris graduation.

Local files contain static content but have not yielded authoritative live ownership data:

- `client/MPBT3025andGILS/MPBT3025/BT3025.vcr` is 61,107 bytes. At file offset `0x3848` it contains a big-endian count of 121, followed by planet records with IDs 1–121 (Solaris is 121); House records follow the planet table. The records include readable planet names and descriptions plus additional binary fields. This serialized client cache is a promising source for the `PlanetInfo` layout and default static values, but it is not a known live server snapshot or packet capture. The additional fields and whether they include usable owner/control values are not decoded yet.
- `client/MPBT3025andGILS/MPBT3025/Data/InnerSphere.k3s` is a ZIP archive of map art: numbered planet icons, `H001`–`H120`/`h121` territory images, and `is_outlines.tga`. These assets explain the 120-territory map visuals, but the archive does not itself supply live owners or control percentages.
- `PlanetInfo` has a serializer at `0x515ca0`, decoder at `0x515d60`, and field formatter at `0x515e30`; `HouseInfo` has a formatter around `0x518910`. The ID0xa8 update serializer/decoder at `0x5239d0`/`0x523a10` process a generic state collection, but no direct reference ties that collection's elements to `PlanetInfo`. The VCR table and A8 network element type therefore remain an unproven match, and the live owner/control values are still unknown. Do not send a guessed list.

The 11:29 experiment reached the map and exercised ID0x91. The server sent ID0x97 with an empty list, but the client remained on the Inner Sphere map and did not send ID0xad. The user reported that territory details appeared on hover; this confirms map-shell interaction, not that the planet's sector lobby loaded. A fuller campaign server will also need persistent Houses, planet ownership/control, rooms and occupancy, pilot progression, and battle-result updates.

The client has a split-message format for oversized game PDUs. Factory case 0x47 at `BT3025.exe:0x4f69a3` constructs the class whose serializer at `0x5027a0` and decoder at `0x502740` handle a one-byte `m_lastInSet`, a uint32 `m_dataLen`, and raw bytes. The Go server wraps an oversized PDU's complete encoded bytes (including its six-byte header) into ID0x47 fragments, marks only the final fragment, and sends each fragment in its own Aries data frame no larger than the 16 KB HOST_ONLINE buffer. These routines confirm the fragment fields, but the client's higher-level reassembly and dispatch behavior remain unverified.

The opt-in `-planet-vcr` candidate is no longer blocked by the size check. It can be sent through the inferred split-message wrapper when the client requests planet ID 0, but the first live attempt terminated the client immediately after the two fragments arrived. The result does not establish whether the fault is in split-buffer handling or in the guessed ID0xa8 payload. Keep this option off until the receive path and list record layout are verified independently.

After Basic Training, the user selected Solaris and clicked **Go To Planet**. The trace `runtime/traces/session-1792432410938502234.log` records ID0x91 with body `00000079 00000000`; the old running server sent no reply and the client stayed on the map. The client class metadata confirms ID0x91 is `JOIN_PLANET_PDU` (class-ID prefix at `BT3025.exe:0x59426c`, serializer at `0x52a700`), ID0x92 is `JOIN_PLANET_FAILED_PDU` (prefix at `0x594234`), and ID0x90 is `COMBAT_SUMMARY_PDU` (prefix at `0x5942a4`). The request body is a uint32 planet ID followed by a length-prefixed string; Solaris is ID 121 and the string is empty. The failed live experiment sent eight zero bytes as ID0x90, then crashed. That was the wrong class and wrong body for a join failure; the exact failure response has not been tested. An ID0x91 echo also left the client on the map. The successful join response sequence remains unknown.

The confirmed message mapping is `REQUEST_PLANET_STATES_PDU` 0xa7 (constructor at `0x523f70`, uint32 serializer at `0x523fc0`), `PLANET_STATE_LIST_UPDATE_PDU` 0xa8 (collection serializer/decoder at `0x5239d0`/`0x523a10`), and `REQUEST_LOBBY_ROOMS_PDU` 0xad (class metadata at `0x593ccc`). The 0xad serializer at `0x521f80` labels its only field `m_planetID`, so its request body is a planet ID, not a counted list. The decoder now returns that ID and the server can answer it with an empty ID0x97 `READY_ROOM_LIST`. No trace to date contains ID0xad. The empty room list therefore has not been shown to be accepted in a live lobby flow, and it contains no sector data. Correct class metadata maps `JOIN_PLANET_FAILED_PDU` to 0x92; 0x90 is `COMBAT_SUMMARY_PDU`.

After the user selected Basic Training, the client displayed the 3D training scene with the tutorial text “This is your radio. It's where you communicate with friends and foes alike. The Tab key will begin messages, and the Enter key will send them. Press Space to continue.” The screenshot was captured at 11:00:05 local time; the server trace was last modified at 10:59:18 and contains no PDU after the lobby requests and heartbeat in that interval. The user completed the lesson and the client returned to the Inner Sphere map shell. No training-specific network exchange was observed, though later mission actions may still use the network.

The user reported very low frame rate during Basic Training. The Wine process maps `wined3d.dll` and `opengl32.dll`, and also has Intel `i915.gem` buffers; `glxinfo -B` reports accelerated OpenGL on Intel HD Graphics 520 (Skylake GT2). This points to Wine's Direct3D-to-OpenGL path on the integrated GPU, not an obvious CPU-only renderer. The host is an Intel Core i5-6300U with two cores/four threads. At 11:08 local, memory was tight (635 MiB available and all 511 MiB of swap allocated), the load average was about 3.9 on four logical CPUs, and short samples showed a high run queue. Host contention and the older integrated GPU can both contribute; the short swap sample did not show sustained swap traffic, so the exact bottleneck is not proven. The game is already in its default 800x600 window. The manual's Video Options recommend Detail=Low for maximum performance and allow reducing Shadow Complexity, Object Complexity, and Terrain Detail. It also documents `O` as the in-combat frame-rate display toggle.

The 11:29 experiment showed that ID0xa8 with an empty state list blanks the map territories, so the default server omits a placeholder update. Trace `runtime/traces/session-1792436091041633956.log` records the client sending ID0x91 with Solaris ID 121 and an empty string; the server replied with an empty ID0x97 list. The client then sent more ID0xa7 requests and heartbeats but no ID0xad. The ID0x97-after-join experiment did not advance the UI and has been removed from source. `internal/protocol/planet_seed.go` extracts the 121-entry planet table from the hash-checked VCR and appends the ID0xa8 sequence/timer fields. The 28,114-byte candidate was sent as two ordered ID0x47 frames; as documented above, the client terminated immediately afterward. The trace and screenshot are evidence of failure, not successful reassembly or valid state data. In the current no-flag run, the trace records the user's repeat of the call-sign/training/planet path: ID0x87, ID0x91 for Solaris, and repeated ID0xa7 polls, but no ID0xa8 update. The captured UI is the map with Solaris selected, **Go To Planet** active, and empty communications and planet-pilot panels; no sector list or staging-area room list has appeared.

Earlier Go unit-test runs passed transport fragmentation/coalescing, length limits, truncated frames, exact greeting bytes, multiple game PDUs, and invalid game lengths. These tests establish framing correctness, not successful lobby startup. Use `./scripts/test.sh` from the current project root to run the reorganized suite.

## Code and running

- `src/server/main.go`: localhost TCP service, trace capture, startup dispatch.
- `src/protocol/aries/frame.go`: transport framing.
- `src/protocol/game/pdu.go`: game framing, greeting, startup replies, and experimental configuration sync.
- `src/protocol/game/planet_seed.go`: opt-in extraction of candidate planet-state records from the exact preserved VCR build.
- `src/protocol/game/pilot.go`: disposable local player and profile builders using recovered serializer order. Pilot identity and profile values are placeholders.
- `client/research/client-inventory.json`: client hashes and named message inventory.
- `client/research/raw/`: objdump/strings outputs used for address references.

Requires Go 1.24 or later, with no third-party Go dependencies. From the current project root, run checks and start the server with:

```bash
./scripts/test.sh
go run ./src/cmd/server
```

The full VCR-derived planet update is opt-in and uses inferred split-buffer
framing. A live attempt terminated the client after delivery, so leave this
flag off during normal client testing. A smaller one-record Solaris variant
avoids fragmentation but still uses an unverified cache record layout. Both
are experiments, not authoritative campaign state:

```bash
go run ./cmd/server -planet-vcr ../client/MPBT3025andGILS/MPBT3025/BT3025.vcr
```

To send only the single Solaris cache record while isolating its ID0xa8 element
layout, use:

```bash
go run ./cmd/server -planet-vcr-solaris ../client/MPBT3025andGILS/MPBT3025/BT3025.vcr
```

While the server is running, use another terminal at the repository root to start the client:

```bash
./client/scripts/launch.sh game aol:127.0.0.1:15000:key:local-dev
```

Stop the client from the repository root with `./client/scripts/stop-wine.sh`.

Client startup takes approximately 20–35 seconds before connecting on this system. Server defaults to 127.0.0.1:15000; `-listen` and `-trace-dir` are configurable. Traces include raw login payloads, so use disposable local keys. Current server accepts any login, has no real authentication or account persistence, and has no gameplay. Its empty room-list reply is experimental. Keep development local. No Git repository is initialized in this directory.

## Verified protocol

Aries header: three little-endian uint32 fields: type, tag, payload length (12 bytes). Server first sends type 0x16 with empty payload. Client answers type 0x15 with login data. Game data uses type 0. In the local login, key length is uint16 LE at offset 322, followed by key at 324 and a terminating NUL; other fields are not fully parsed.

Game PDU header: big-endian uint16 ID followed by uint32 **total** length including the 6-byte header. Multiple PDUs can share an Aries frame. Cross-frame game-PDU reassembly is not implemented.

Verified HOST_ONLINE (ID 29) complete wire bytes:

```
001d0000000e0000010502024000
```

Body: reserved-word count uint16=0, four version bytes 1.5.2.2, buffer size uint16=16384. An earlier uint32 reserved-word count caused a client deserialize exception; the current uint16 greeting works.

Client startup response:

| ID | Body hex |
| --- | --- |
| 0x09 | 010502020000000080000000f7465db7 |
| 0x33 | 0000 |
| 0x37 | 00000000 |
| 0x35 | 00000000 |
| 0x31 | 00000000 |
| 0x43 | 00000000 |
| 0x26 (LIST_ROOMS) | empty |

Heartbeat 0xa3 has an empty body. Signed game int32 values use unsigned bias/XOR 0x80000000; strings use uint32 BE byte length followed by bytes, no NUL.

## Reverse-engineering anchors

Addresses are preferred virtual addresses in the inventoried binaries, not file offsets.

- `aries.dll` 0x10001491 handles login request; 0x10001545 constructs login response.
- `BT3025.exe` game encoder 0x511960; decoder 0x5119d0; exact consumed-length check 0x511a43 throws via 0x530d7d.
- HOST_ONLINE encoder 0x50c260; reserved-word list 0x4f99b0; host parameters 0x503520.
- Primitive writers: bool 0x4f73e0, u8 0x4f7400, u16 0x4f7460, u32 0x4f74c0, biased signed32 0x4f74e0, string 0x4f7710/0x4f7620.
- Player update ID0x3c: data serializers at 0x4fce40 and 0x51b690; handler at 0x4df150 sets connection flag +0x1f8. The base emits a counted container, then a byte and bool; the derived serializer emits a string, counted container, byte, three uint32s, bool, and a string. The disposable fixture uses empty containers and zero-valued fields. Live WineDbg inspection reached the `BTPlayerInfo` decoder and passed the exact-length check.
- Profile update ID0x3d: data serializers at 0x4fd800 and 0x51c350; handler at 0x4df0e0 sets connection flag +0x1f9. The base writes six counted containers, two strings, two bools, one byte, and two uint32s. The derived serializer writes five counted containers, one string, two bytes, one bool, and five uint32s; the `+0x94` uint32 is serialized after `+0xbc`. The first body omitted that word and failed the client's exact-length check. The current 101-byte body passes the live decoder and advances the UI beyond the upload screen. Its profile and mech values are still placeholders.
- Host-online handler 0x4df380 sends startup requests including LIST_ROOMS.
- Static configuration ID0x86 encoder 0x52e210, decoder 0x52e270, handler 0x4de4a0. Body: bool isSynched, five counted tables (each uint32 count), sequence byte, total 22 bytes when empty. The previous “four tables + revision” interpretation was wrong: serializer inspection shows five table writers and no revision field. Empty unsynchronized sequence-0 config receives 0xa4 sequence 0; a final synchronized update follows. This exchange is accepted, but does not yet advance the UI.
- Config handler with isSynched=false merges table deltas; sequence 1 first clears cached tables. It sends response ID0xa4 containing sequence, then sets flag +0x1e5. isSynched=true can emit event 0x80 and release deferred profile/qualification events when +0x1e5 is set. Complete exchange remains unproven; do not clear tables without supplying replacements.
- `BT3025.vcr` is persisted serialized state, not an established packet recording. Encode/decode 0x4dd750/0x4dd940; base serialization 0x4f1b40. Contains configuration tables that may help bootstrap. No extractor implemented.

`client/.runtime/dump_code.py` is a local targeted objdump helper, accepting hex start[:size]. `client/.runtime/capture.py` captures the Wine desktop to `client/.runtime/client.png`. Both are ignored experimental helpers. Avoid naming a helper `dis.py`, which shadows Python's stdlib module.

## Recommended continuation

1. Treat the first milestone as “map shell reached,” not “planet lobby entered.” Keep the trace as evidence and avoid describing this as persistent character creation.
2. Recover the actual response sequence after the ID0x91 Solaris request so the client enters a Solaris city lobby. The request's second field is `m_password` (empty in the observed request). The zero-result ID0x92 trial displayed a failure dialog, and the inventory contains no separately named join-success PDU. Follow the client’s world-session/redirect path and identify the state it needs after the join.
3. Keep VCR-derived ID0xa8 state disabled. Its element wire shape and first-field lookup-key role are known, but state fields and valid starting control values remain unresolved. The 11:25 run sent the 121-entry numeric key-presence probe; the client stayed on the map and no owner projection was established. See the latest trace and screenshot below. Describe keys numerically until their planet-ID meaning is proven. The manual's world rules remain acceptance criteria, not a substitute for the missing state data.
4. The current `"Local Pilot"`, no-House profile, and empty config deltas are placeholders. `BT3025.vcr` contains readable local names and descriptions but is not an established packet capture or authoritative server state; no extractor exists yet.
5. Preserve a trace and screenshot after each server change. The latest no-reply join trace is `runtime/traces/session-1792520472103837304.log`, with the after-wait capture `client/.runtime/captures/solaris-join-20261020-112510-after-wait.png`. The earlier October 20 startup sessions ended before the client sent ID0x91, so they do not replace this map-only evidence.

Earlier architectural research is in `research/notes/server-emulator-research.md`. Go remains the agreed implementation language: standard-library networking, explicit message types, small packages, and readable code. Runtime Wine/client assets are separate from the portable emulator.

Environment cleanup: invoking `winedbg --help` accidentally created a fresh default `~/.wine` during investigation. Its processes were stopped and that newly created prefix moved to `client/.runtime/accidental-default-wine-prefix` for reversible cleanup. The project prefix remains `client/.runtime/wine32`.

### Player-only refresh trial (14:30–14:32 local)

To isolate the map corruption seen when both post-creation records were sent,
this run kept the `0x3c` player refresh and omitted the `0x3d` profile refresh.
The startup `0x3c`/`0x3d` placeholders remain unchanged. Trace
`session-1792445396376423182.log` records the saved `Subvoltic` create request,
the `0x88` completion, then only a 47-byte `0x3c` refresh. The user reported
that the training menus were working and that the news ticker was scrolling.
After **Go To Planet**, the client sent `0x91` for Solaris (planet 121); the
server saved the location but sent no response. Repeated `0xa7` planet-state
polls and `0xa3` heartbeats followed, with no `0xad` lobby-room request.

The map remained visible without the clipped Solaris-description overlay from
the prior no-`0xa8` run that sent both post-creation refreshes. The captured
screen has a Pilot Information panel with location and statistics shown as
unavailable (`???`), consistent with the omitted profile data. This points to
the post-creation `0x3d` as a likely source of the earlier visual corruption,
but the result is not conclusive: the profile-only trial has not been run.
Screenshot: [`evidence/solarismap-player-update-only.png`](../../../../MPBT3025/client/.runtime/captures/server-evidence/solarismap-player-update-only.png).

This trial confirms that recording a successful `JoinPlanet` in server state
does not trigger the client transition by itself. The client remained on the
map, so the missing lobby transition is still the next blocker. The earlier
empty `0x97 READY_ROOM_LIST` reply also failed to advance the client; a useful
response likely needs an additional success update before the client requests
rooms. The room-list serializer at `0x5287a0` writes the counted 4-byte list
at `+0x08`, the ordered tree size at `+0x28`, then each tree key/value pair
from `+0x20`. The known-valid empty eight-byte body represents an empty list
and empty tree; it is not a populated room encoding.

The source now leaves the post-creation profile refresh disabled while this
trial is investigated. Startup still sends the placeholder profile so the
client exits its upload screen. Build succeeded with the bundled Go toolchain;
no Go tests were run during this client-only experiment.

### Malformed populated-room-list probe (14:39 local)

To see whether the client would advance on room data, the server briefly sent
an 84-byte `0x97 READY_ROOM_LIST` immediately after accepting Solaris
`JOIN_PLANET`. The client showed a Microsoft Visual C++ Runtime Library dialog
with “abnormal program termination” as soon as **Go To Planet** was clicked.
The trace is [`session-1792445946435812621.log`](../../runtime/traces/session-1792445946435812621.log)
and the captured screen is
[`evidence/solaris-one-room-list-crash.png`](../../../../MPBT3025/client/.runtime/captures/server-evidence/solaris-one-room-list-crash.png).

This was an invalid format probe, not evidence that a valid populated list is
unsafe. The ID0x97 serializer at `BT3025.exe:0x5287a0` does not serialize a
top-level count followed by `BTRoomInfo`. It writes a counted 4-byte list at
object offset `+0x08`, the ordered tree size at `+0x28`, then each tree node's
custom string-like key and 32-bit value. The decoder at `0x528810` confirms the
same outer sequence. The map's meaning and how either collection relates to
room records remain unknown. Our candidate placed a `BTRoomInfo` where the
list expects a 4-byte element and omitted the tree count and entries; that does
not match the serializer and likely desynchronized decoding before the crash.

The live `JOIN_PLANET` handler no longer sends this guessed `0x97` response.
It persists the pilot's destination and waits for an observed client request.
The known-valid empty `0x97` remains only as the reply to `REQUEST_LOBBY_ROOMS`
(`0xad`); it still has not been shown to trigger the map-to-lobby transition.
Build was re-run after removing the unsafe response; no Go tests were run.

### Clean join baseline after removing the room-list guess (14:42–14:47 local)

The client reconnected from its saved `Subvoltic` call sign and reached the
map without a crash or visual corruption. After the user clicked **Go To
Planet**, trace
[`session-1792446163786460377.log`](../../runtime/traces/session-1792446163786460377.log)
records `0x91` for Solaris (121), then repeated `0xa7` requests for planet 0
and `0xa3` heartbeats. The server sent no response to the valid join; the user
confirmed that the client stayed on the map. No `0xad REQUEST_LOBBY_ROOMS` was
sent. The screenshot is
[`evidence/solaris-clean-baseline-map.png`](../../../../MPBT3025/client/.runtime/captures/server-evidence/solaris-clean-baseline-map.png).

This is the current safe baseline. Removing the malformed `0x97` candidate
avoids the crash, but does not advance the client. The next protocol target is
the correct state or success update that takes the client from the map into
the planet/sector flow; a lobby room list is not requested yet.

### Recovered 0xa8 element codec and safe schema probe (14:51 local)

Disassembly of the `0xa8 PLANET_STATE_LIST_UPDATE` collection decoder at
`BT3025.exe:0x4e58a0` shows it constructs each list element with
`0x517c20`, then dispatches the element decoder at `0x517d40`. The matching
serializer is `0x517ce0`. **Correction (16:47):** the first two element fields
are uint32 values, not strings. Their wrapper vtable is `0x586240`; its
serializer at `0x513b60` writes only the four-byte value at object `+0x04`,
and its decoder at `0x513b80` reads one uint32. The remaining layout is a
uint32 count followed by that many uint16 values, a uint32 count followed by
that many bytes, and one final byte.
The enclosing PDU ends with the uint32 sequence number and uint16
seconds-until-shift field. This layout differs from cached `PlanetInfo`, which
explains why copying VCR cache records into the list was unsafe.

The executable does not identify the element fields' meanings in this codec,
and we have not found a reliable public BT3025 protocol schema. The Go server
now has an encoder for the recovered shape and an opt-in
`-planet-state-schema-probe` flag. The original version encoded a Solaris
string and was malformed under the corrected layout. It is now a numeric
one-entry probe with first value 121, second value zero, empty arrays, and a
zero flag (27-byte PDU body). The revised packet was sent in the later 16:49
trial: the client stayed open through the subsequent Solaris join but remained
on the map. This confirms decoder safety for that packet shape, not field
semantics, authoritative Solaris data, or a lobby transition. The flag is off
by default and mutually exclusive with the VCR experiments.

An Internet search for the exact `PLANET_STATE_LIST_UPDATE_PDU` and its
record names did not surface a BT3025 wire-format source. The separate Solaris
1996 server and other Aries-era multiplayer services remain useful for service
boundaries such as persistent world state, lobby presence, and combat, but
their inner messages do not establish this client's record layout. For this
codec, local executable disassembly is the evidence.

### Malformed string-shaped planet-state probe and client failure (14:56–14:58 local)

This client was launched with the required Aries startup argument:
`./client/scripts/launch.sh game aol:127.0.0.1:15000:key:local-dev`. It
connected successfully, completed startup/config sync, replayed the saved
`Subvoltic` create request, and reached the map. The trace
[`session-1792446987716275832.log`](../../runtime/traces/session-1792446987716275832.log)
records the user clicking **Go To Planet**: `JOIN_PLANET` (`0x91`) for Solaris
(ID 121), followed by a global `REQUEST_PLANET_STATES` (`0xa7`) for ID 0. With
the original `-planet-state-schema-probe` enabled, the server sent a malformed
40-byte `PLANET_STATE_LIST_UPDATE` (`0xa8`) containing the bytes of `Solaris`
where the client expected numeric fields.

The user reported that the map stayed open and then clicked **Game** in the
upper-left menu; the client displayed a Microsoft Visual C++ Runtime Library
“abnormal program termination” dialog. The captured screen
[`planet-state-schema-probe-runtime-error.png`](../../../../MPBT3025/client/.runtime/captures/server-evidence/planet-state-schema-probe-runtime-error.png)
shows the map's House territories rendered black, Solaris selected, and the
runtime dialog over the map. The Wine log
`client/.runtime/logs/game-20261019-145609-303245.log` ends with
`allocate_virtual_memory out of memory ... size 40010000` (a 1 GiB allocation).
At capture time the game process and TCP connection were still present behind
the dialog; the trace has no client PDU after the `0xa8` response.

This is not a successful planet transition. The packet was malformed: the
client expected two numeric fields, but the probe wrote a string length and
text. Subsequent bytes were therefore decoded at the wrong offsets. The OOM
line is direct evidence for a local allocation failure; it does not prove
whether the one-entry response triggered it, whether
the **Game** menu path exposed an earlier fault, or whether host memory pressure
contributed. A nearby host snapshot reported about 1 GiB available and swap
nearly full. Do not use this probe in normal runs; the flag remains opt-in and must stay off
until the state fields and valid values are identified. The successful client
startup also confirms the earlier upload screen was caused by omitting the
`aol:127.0.0.1:15000:key:local-dev` argument, not by the lobby handler.

After capturing the error, I stopped the Wine session and restarted the server
without experimental flags. A fresh launch with the correct `aol:` argument
connected at 15:00:30, completed Aries login/config synchronization, and
displayed the intro page with **Next**. No character-creation, join, or
planet-state request has occurred in this safe run yet. This re-confirms that
the upload screen is a client-launch-stage issue when the Aries startup
argument is absent; it is separate from the still-unimplemented planet-to-lobby
transition.

### Latest clean map trace and EXIT_ROOM handling

In the later clean client session, the user reports the top menus render again.
Trace [`session-1792447230304058790.log`](../../runtime/traces/session-1792447230304058790.log)
records the client creating/reusing `Subvoltic`, sending `JOIN_PLANET` (`0x91`)
for Solaris (121), polling global planet state (`0xa7`, ID 0), and then sending
`EXIT_ROOM` (`0x17`) with a four-byte empty-string body. The server previously
ignored `0x17`; no room-list request (`0xad`) follows, and the client remains on
the map. An earlier note in this investigation misidentified `0x17` as
`FIND_PLAYER`; the client's dispatch table and message class strings identify
`0x17` as `EXIT_ROOM` and `0x18` as `FIND_PLAYER`.

The server now parses the length-prefixed exit-room name, removes the active
pilot from any mission-room seat, and sends a same-class `0x17` room update as
an acknowledgement. The trace shows only a client request, so treating this
same-class update as the expected acknowledgement is an explicit trial
hypothesis. The handler is idempotent for pilots who are not in a mission room.
Planet joins also now remove stale mission-room seats before updating location,
and rejoining the same city preserves its city assignment instead of silently
moving the pilot to city 1.
Keep the client open and inspect the next map-to-Solaris attempt: if it still
does not change screens, capture whether it sends another message after this
reply before trying any populated `0x97` or `0xa8` payload.

The next clean run remained on the map. Trace
[`session-1792449434067515883.log`](../../runtime/traces/session-1792449434067515883.log)
shows character creation/reuse for `Subvoltic`, `JOIN_PLANET` for Solaris
(121), and repeated global `0xa7` planet-state polls. It ends without `0x17`
or `0xad`, so this run does not exercise the trial `EXIT_ROOM` acknowledgement
and provides no evidence that the client attempted to enter the city lobby.

The Player's Guide says a pilot who has completed training should click **Go To
Planet** and appear in one Solaris city lobby; it separately describes the
sixteen-city selection prompt for experienced pilots returning to Solaris.
That supports using a direct cadet-to-city-lobby transition as the acceptance
target, rather than inserting an Inner Sphere sector-selection step. See the
[MPBT:3025 Player's Guide v1.4.5, pages 8–10](https://device.report/m/9776385816d7385322976ddba35c95e5bb7b7f7fc15658ebbc6d92bf324cdbc1.pdf).

### Design-spec research (SDS reference included)

The supplied 1998 Software Design Specification is now kept in
[`reference/sds/btech3025-sds-v1.1a.doc`](../../reference/sds/btech3025-sds-v1.1a.doc),
and the player manual is in
[`reference/manual/Multiplayer-Battletech-3025_Manual_Win_EN.pdf`](../../reference/manual/Multiplayer-Battletech-3025_Manual_Win_EN.pdf).
The extracted SDS text and earlier analysis notes remain in the separate client
research area. These documents provide product-behavior evidence, not packet
definitions; protocol findings also rely on server traces, implementation
fixtures, and observed behavior.

The SDS makes a Solaris city lobby a persistent player location with user
presence and navigation, and describes the Solaris Starport → Tram → five
Arenas flow. It also requires the host to preserve location across login and
world data to refresh within five seconds (§§4.1.3, 4.1.6, 4.12). That supports
the location/world model but does not establish which PDU causes this client
to switch from the map view.

One isolated transition trial sent the already-used `0x3c UPDATE_PLAYER` after
saving the pilot's destination. Trace
[`session-1792450142919844307.log`](../../runtime/traces/session-1792450142919844307.log)
confirms a `JOIN_PLANET` for Solaris followed by the 46-byte `0x3c
UPDATE_PLAYER` refresh. The client stayed on the map, then sent only its
heartbeat. No lobby request followed. This rules out the existing
player-refresh message by itself as the missing transition signal, so the
duplicate refresh has been removed.

The client binary's profile metadata names an `m_lastPlanet` field, and the
decoded profile serializer places it at object offset `+0xd4`. A field-focused
trial sent a `0x3d UPDATE_PROFILE` after the saved Solaris join with that field
set to `Solaris`. Trace
[`session-1792450439664245208.log`](../../runtime/traces/session-1792450439664245208.log)
shows the client sent `JOIN_PLANET` (`0x91`, planet 121), the server persisted
the location, and then sent a 112-byte profile PDU. The client produced a
Microsoft Visual C++ Runtime Library “abnormal program termination” dialog
while still on the map. The trace contains no client message after the profile
update.

This is a strong temporal correlation, not proof that `m_lastPlanet` alone
caused the termination: the other profile fields still contain empty
development values, and the client did not send a diagnostic packet. The
post-join profile update and its planet-populated builder have therefore been
removed. Startup's previously used empty profile remains unchanged. Do not
retry the populated profile update without decoding the complete profile
contract. The server can safely record the pilot's Solaris location, but the
map-to-lobby transition is still unresolved; a successful `JOIN_PLANET` is not
evidence that the client entered a lobby.

After removing that update, the user repeated the full Solaris path. The client
stayed on the map and did not crash. Trace
[`session-1792450621849546199.log`](../../runtime/traces/session-1792450621849546199.log)
shows the server receiving `JOIN_PLANET` for Solaris, followed by client
`REQUEST_PLANET_STATES` (`0xa7`) polls for planet 0 and `0xa3` heartbeats. The
server sent no reply to either request, and there was no `0xad
REQUEST_LOBBY_ROOMS`. This confirms the rollback removed the crash on this run,
but leaves the screen transition unresolved. The repeated global-state poll is
the next observed server request to investigate.

The user clarified the progression target: cadets go to Solaris, fight there
until eligible to choose a faction, and do not choose a House before entering
the Solaris lobby. This agrees with the Player's Guide's new-pilot path and
remains distinct from later Inner Sphere travel.

### Comparative ARIES redirect trial

The earlier-game reference repository was shallow-cloned read-only to
`/home/sebulba/MPBT-Solaris-reference` from
[`kenhuman/mpbt-server`](https://github.com/kenhuman/mpbt-server) at commit
`4c12b96` on 2026-10-21. It is AGPL-3.0; no source was copied into this
project. Its useful comparison is architectural: a lobby login can redirect
to a second world connection, and room presence is initially seeded as a batch
containing self and peers before incremental arrivals. Its inner frames and
command IDs belong to the 1996 `MPBTWIN.EXE` client and are not BT3025 wire
evidence.

Internet research located the public [`kenhuman/mpbt-server`](https://github.com/kenhuman/mpbt-server)
emulator for the earlier *MPBT: Solaris* client. Its README documents a 120-byte
ARIES `REDIRECT` (`0x03`) with three 40-byte fields, then a second TCP connection
to a world server where the client logs in again. That server acknowledges the
login, sends a welcome marker, waits for the client's capability frame, then
sends a loading-cursor command, scene initialization, room presence, a welcome
broadcast, and a normal-cursor command.
Our shipped `aries.dll` also dispatches an outer message type 3 through its
connection path, so this is a credible architecture clue. The clients and inner
protocols differ: the older game uses command frames where 3025 uses typed
PDUs. This does not prove that BT3025 redirects after `JOIN_PLANET` or that it
uses the same world bootstrap messages.

An opt-in `-planet-redirect` experiment now sends that documented outer redirect
to a separate local listener on port 15001 after persisting a Solaris join.
The second listener uses the existing handshake only to capture what the 3025
client asks for next; no experiment is enabled by default. This tests whether
the 3025 client expects the same two-stage connection at this progression
boundary without mixing the old game's inner game-command framing into 3025.

The live redirect trial did open the second TCP connection. Trace
[`session-1792450988031937504.log`](../../runtime/traces/session-1792450988031937504.log)
records the initial login, `JOIN_PLANET` for Solaris (121), and the 120-byte
outer redirect (`0x03`) to `127.0.0.1:15001`. The client then logged in again;
[`session-1792451016339743396.log`](../../runtime/traces/session-1792451016339743396.log)
records its 325-byte second `LOGIN`, the normal host greeting and startup
configuration exchange, then `0xa7` planet-state polls and `0xa3` heartbeats.
It did not send `0xad REQUEST_LOBBY_ROOMS` or another `0x91 JOIN_PLANET`.

The user saw the Inner Sphere map with Solaris selected and the tutorial `Next`
panel visible, and reported that the interface was visually bleeding again.
This is not a successful lobby transition. The server sent the same generic
bootstrap and placeholder pilot/profile updates on the redirected session;
that may be inappropriate for a world-session login, but this single trial
does not prove it caused the visual problem. The evidence confirms that outer
redirect framing reaches a second Aries login, not that BT3025 uses the older
game's redirect architecture to enter a planet lobby. Keep `-planet-redirect`
off for ordinary runs until the distinct world-session startup is recovered.
The normal server has been restarted without experimental flags.

### BT3025 generic occupant-list PDU mapping (2026-10-21)

The executable's fixed-width PDU-name table and class descriptors confirm that
the generic occupant PDUs share the same inner 16-bit ID namespace as the
existing `LIST_ROOMS` (`0x26`) / `LIST_ROOMS_RESPONSE` (`0x27`) handling:
`LIST_OCCUPANTS` is `0x24`, `LIST_OCCUPANTS_RESPONSE` is `0x25`, and
`UPDATE_OCCUPANTS` is `0x3b`. The name lookup at `BT3025.exe:0x503bb0` indexes
the table by the PDU's 16-bit ID, with 64-byte name slots.

The `LIST_OCCUPANTS_RESPONSE` descriptor at `0x5932d8` points to serializer
`0x50a600`. The request serializer at `0x50a8f0` writes its `m_roomID` string;
the response serializer writes its `m_roomID` string followed by an occupant
vector at object offset `+0x34`. The `UPDATE_OCCUPANTS` descriptor at
`0x592e08` points to serializer `0x506a10`, which serializes an occupant vector
at `+0x10`. Both vectors use helper `0x512360` to write the count and serialize
each entry. The `BTPlayerInfo` virtual serializer at `0x51b690` matches the
record shape used by the game's player-update PDUs, making it the current
candidate occupant entry type; the exact `0x25` wire body still needs a
captured request/response exchange.

These are room-level roster messages, not evidence for the map's planet roster
or the missing `JOIN_PLANET` success transition. The current two-client map
traces have not shown `LIST_OCCUPANTS` (`0x24`), so do not send an unsolicited
`0x25` response. The client still has not sent `REQUEST_LOBBY_ROOMS` (`0xad`)
after `JOIN_PLANET`.

### Rechecking the apparent join dispatcher

The earlier note that `BT3025.exe:0x41d1a0` handles the `JOIN_PLANET` PDU was
incorrect. That function switches on the first 32-bit value of an internal
message object: it subtracts 2, indexes a byte table at `0x41e3cc`, then uses
the result to select a handler from the table at `0x41e398`. The value 0x91
maps through table entry 0x0a to `0x41d469`, which calls `0x41eb00` to set a
timer. This is not enough to identify the internal value as the wire PDU ID,
and the branch does not establish a join-success response contract. The former
claim that `0x91` maps to `0x41dfdb` should not be used; that target is table
entry 9, not entry 10. The executable disassembly remains at
[`BT3025.exe.disassembly.txt`](../../../../MPBT3025/client/research/raw/BT3025.exe.disassembly.txt).

The wire evidence remains the class names and serializers for
`JOIN_PLANET_PDU` (0x91) and `JOIN_PLANET_FAILED_PDU` (0x92), plus the live
request trace. The recovered PDU-name inventory contains the request and a
typed failure, but no separately named successful-join PDU. That makes a
server push of other world state or a service redirect plausible, not proven.
The successful transition message is still unknown. Do not echo 0x91 or invent
a 0x92 failure body to force a screen change. Continue by locating the actual
network PDU receive dispatch and determining what state the client needs after
a successful Solaris join.

After restoring the normal listener and relaunching with the required `aol:`
argument, the user repeated the Solaris path. Trace
[`session-1792451313844083080.log`](../../runtime/traces/session-1792451313844083080.log)
records call-sign reuse, `JOIN_PLANET` for ID 121, then `0xa3` and global `0xa7`
polling. The server sent no planet-state or transition update, and the user
confirmed the client remained on the map. This re-establishes the safe baseline
after the redirect experiment; it does not resolve the join response contract.

### Planet-state consumers found in the map client

The preserved disassembly contains two explicit receive checks for wire PDU
`0xa8`: `BT3025.exe:0x43ba95` and `0x460ee4`. The first obtains the decoded
PDU's list head at `+0x10` and iterates its entries through `0x43bd70`, then
calls `0x437610` for each entry. The second looks up a key from the current
world object at `+0xa4` in the decoded list using `0x4290c0`, then uses the
returned entry in a longer UI update path. This second path is embedded among
combat UI labels including “Lance Alpha” and “The Defending Force”; it should
not be assumed to run on the map screen. These are concrete UI consumers of
the state list; the `0xa8` response is not merely an acknowledgement of
`0xa7`. This does not yet identify the required element values or a join
transition. The earlier one-entry schema probe used an incorrect string layout;
it did not establish whether a structurally valid element is safe.

### Zero-result join-failure trial (16:29 local)

An isolated test sent a correctly typed `0x92 JOIN_PLANET_FAILED` with a zero
result and an empty string after a valid Solaris `0x91 JOIN_PLANET`. Trace
[`session-1792452518054888615.log`](../../runtime/traces/session-1792452518054888615.log)
records the 14-byte response. The client remained on the map and displayed a
ComStar dialog reading “Failure Received!”, `MessageID=146`, `StatusCode=0`,
and empty `ReasonText`. The captured screen is
[`join-planet-zero-result-failure.png`](../../../../MPBT3025/client/.runtime/captures/server-evidence/join-planet-zero-result-failure.png).
Unlike the zero-result `0x88` create-character completion, a zero-result
`0x92` is still treated as a failure display. This rules out `0x92` as the
successful join transition in this client. The opt-in probe was removed from
the server after the trial.

### Full named-list planet-state trial (16:32 local)

To test whether the one-entry `0xa8` probe failed because the map needed a
complete list, an opt-in run sent 121 entries in planet-ID order using names
from the saved world. Every entry used the then-assumed string layout, left
the second string and both arrays empty, and set the flag to zero. The encoded PDU was
2,999 bytes, below the client buffer size. Trace
[`session-1792452706814456490.log`](../../runtime/traces/session-1792452706814456490.log)
shows the client requesting global planet states (`0xa7` with ID 0) and the
server sending this single `0xa8`. The game process exited with status 3
immediately afterward, before call-sign or Solaris selection, and sent no
further PDU. No useful Wine diagnostic was written to that run's log.

The client expected uint32 values where this packet wrote string lengths and
text. This malformed encoding is a sufficient explanation for the failure;
the trial cannot establish anything about required planet count or field
semantics. The opt-in full-list probe was removed.

### House-name second-string trial (16:44 local)

The next isolated `0xa8` trial kept all 121 planet names but populated each
entry's second string with its prototype owner House name (`none` for Solaris).
Both arrays and the flag stayed zero. The encoded PDU was 3,675 bytes. Trace
[`session-1792453294284181281.log`](../../runtime/traces/session-1792453294284181281.log)
shows the global `0xa7` request and single `0xa8` response. The client reached
the map and call-sign pane, then displayed the Microsoft Visual C++ Runtime
Library “abnormal program termination” dialog. Screenshot:
[`planet-state-house-probe-runtime-error.png`](../../../../MPBT3025/client/.runtime/captures/server-evidence/planet-state-house-probe-runtime-error.png).
The Wine `+seh` log for this run records `allocate_virtual_memory out of memory`
for a `0x40010000` allocation just before the C++ runtime exception. This
is consistent with the subsequently identified numeric/string encoding error.
This trial cannot establish the second field's meaning. The probe was removed
and normal startup restored without `0xa8` replies.

### Correct numeric element layout and live one-entry result (16:49 local)

Further disassembly of the first two field serializers in the `0xa8` element
showed that each writes a uint32, not a length-prefixed string. The updated Go
encoder now writes two uint32 values, a counted uint16 array, a counted byte
array, and a trailing byte. The preceding named-list crashes were malformed
wire data: name lengths and bytes shifted the client's subsequent array counts.

The corrected opt-in one-entry probe sent first value 121 and zero for every
other field. Trace
[`session-1792453643556995813.log`](../../runtime/traces/session-1792453643556995813.log)
records a 33-byte `0xa8` after a global `0xa7`, followed by normal heartbeat,
another `0xa7`, Subvoltic creation, and Solaris join. The user confirmed the
client stayed open on the Inner Sphere map. This establishes that the numeric
element is safe to decode in this case. It does not establish the field
semantics or a lobby transition.

### Full numeric-list cardinality trial (16:56 local)

An opt-in `-planet-state-full-probe` now sends 121 structurally valid numeric
entries, with first values 1 through 121 and all other fields zero. Its encoded
PDU is 2,073 bytes. This isolates list completeness from the still unknown
ownership and sector fields. The next trace,
[`session-1792454214877242901.log`](../../runtime/traces/session-1792454214877242901.log),
contains startup/configuration and heartbeats, but no global `0xa7` request or
`0xa8` response. That session did not exercise the probe. The later October 20
run exercised the 121-entry packet; its trace and unchanged map are recorded
in the latest handoff entry below.

Reviewing the newer October 20 captures found three startup sessions
([`session-1792508819539932789.log`](../../runtime/traces/session-1792508819539932789.log),
[`session-1792510229117640325.log`](../../runtime/traces/session-1792510229117640325.log),
and [`session-1792511031958442614.log`](../../runtime/traces/session-1792511031958442614.log))
that end after call-sign creation and the global `0xa7` request. They contain
neither a Solaris `0x91` join nor a server `0xa8` update, so they are not
results for this trial. The client and server are currently stopped, as
requested.

Static follow-up on the `BTProfileInfo` formatter at `BT3025.exe:0x51c510`
also recovered the labels `m_lastLogin`, `m_mechPoints`, `m_salary`,
`m_lastPlanet`, and `m_qualificationBitField`. The known `m_lastPlanet` offset
remains `+0xd4`; the formatter names alone do not establish valid values or
make the failed post-join profile refresh safe. Keep the startup fixture and
post-join omission unchanged until the full profile contract is decoded.

### 0xa8 record lookup key recovered

Further inspection of the counted-container decoder at `0x4e58a0` shows that
it inserts decoded records into a keyed tree using the first serialized
`uint32` (the entry object's value at `+0x8`). The second consumer at
`0x460ee4` performs a lookup in that tree using a `uint32` read from the
current world object at `+0xa4`; the lookup helper's comparator is at
`0x42fcd0`. This is stronger evidence than the earlier generic list iteration:
the first wire value is a lookup key, and the client later requests the key
associated with its current world object. The field is still unnamed by the
binary metadata, so call it a record key until the `+0xa4` world-object field
is independently identified. Its use is consistent with a planet ID, but
does not prove that label.

This makes the existing 121-entry numeric probe (keys 1 through 121) a useful
cardinality/key-presence trial, but it still cannot populate ownership,
control, or sector state because the second scalar, arrays, and flag have no
recovered meanings. The probe was not exercised in the October 20 captures,
and remains off by default. The next useful client trace must reach the
Solaris `0x91` join and the global `0xa7` poll while the numeric probe is
enabled; only a captured UI change or subsequent `0xad` request would add
evidence about the transition.

### 0x97 ready-room list wire shape recovered statically

The `READY_ROOM_LIST` serializer at `BT3025.exe:0x5287a0` calls a generic
container codec for its member at `+0x08`, then writes the size of the ordered
tree at `+0x28`, then walks the tree at `+0x20` and serializes each node's
`+0x0c` key followed by its `+0x24` value. The `+0x08` container writes a
count followed by 4-byte elements using the uint32 element callbacks
(`0x513b60`/`0x513b80`, decoder primitive `0x4ed2c0`); these are not inline
`BTRoomInfo` records.

The tree key is a custom string-like object: its serializer/decoder are
`0x4f8710`/`0x4f8740` under vtable `0x591508`, and its formatter prints string
data. The comparator at `0x4f8660` compares the key's byte content and length
before a secondary scalar at key offset `+0x14`. On the wire, the key
serializes that uint32 discriminator followed by a length-prefixed string; the
discriminator's meaning is unknown. The value is a 32-bit wrapper whose
serializer at `0x513b60` writes its `+0x04` field; its formatter labels it
`m_value`. This recovers the wire shape as a
counted list of 32-bit elements followed by a counted tree of uint32
discriminator, length-prefixed string, and uint32 value entries. It does not
show what either collection means or how list elements refer to room records.
The server now has a neutral typed encoder for this structure, but still sends
the empty form until the list and tree meanings are known well enough to supply
real values.

`BTRoomInfo` has a separate 0xb8-byte client object codec at `0x51f1f0` and
`0x51f320`. Its formatter at `0x51f460` exposes labels including
`m_roomType`, `m_parentLobby`, `m_totalOccupancy`, `m_minRank`, `m_maxRank`,
`m_desc`, `m_planet`, `m_destroyEmpty`, `m_visible`, seat counts, and level
fields. That record codec is not directly called by the `+0x08` list's
4-byte-element callbacks. So the earlier crash probe was structurally wrong:
it put a `BTRoomInfo` body where the client expects 4-byte list elements, and
omitted the tree count and entries. This static finding does not establish how
the auxiliary tree relates to room records, nor a valid populated room-list
body. No new live probe was run; the game and server remain stopped.

### Full planet-state cardinality check (run 2026-10-20)

The remaining bounded data question is whether the map's lookup requires all
121 record keys. The opt-in `-planet-state-full-probe` sends 121 structurally
valid numeric records keyed 1 through 121, with every other record field zero.
Only the key's lookup role is supported by the binary evidence; the zeroed
fields are not authoritative planet state and may change map rendering. This
check can determine whether the client needs a complete key set before it can
use Solaris state, but it cannot by itself prove the field meanings.

The 121-entry probe was run with the bundled Go toolchain:

```bash
./runtime/toolchain/go/bin/go run ./cmd/server -planet-state-full-probe
```

The client was launched from the project root with its required Aries address:

```bash
./client/scripts/launch.sh game aol:127.0.0.1:15000:key:local-dev
```

The server sent the 2,073-byte 121-entry snapshot after the client's first
global `0xa7` request. The user first reported remaining on the map, then
reported that the map started “doing stuff” after the reply. The client stayed
connected and sent further `0xa7` requests for planet 0. The captured trace has
no subsequent `0xad` lobby-room request, so the probe changed map behavior but
has not yet demonstrated a Solaris lobby transition. The visual change has not
been identified precisely, and this environment's CUA surface exposes no app
window for a screenshot.

Next, while the same client remains open, select Solaris and click **Go To
Planet** once if that has not already been done after the map changed. Check the
new trace for `0xad` and record whether the client stays open or changes screens.
Keep this run separate from either VCR flag and from any populated `0x97`
response.

### Automated local checks and CLI evidence capture (2026-10-20)

`cmd/server/main_test.go` now drives the server through a synthetic Aries peer:
login/bootstrap, pilot creation, Solaris `0x91`, the safe 121-key numeric
`0xa8` probe, and a `0xad` request answered only with the empty `0x97` form.
This checks server protocol behavior and trace output; it does not prove that
the Windows client accepts a lobby transition.

Run the current suite from the project root with `./scripts/test.sh`.
`client/scripts/capture-test-evidence.sh [trace.log] [screenshot.png]
[trace-offset]` captures the game window through X11 and summarizes the trace,
optionally from a saved byte offset. It does not start or stop the processes or
send mouse input.

For the live full-cardinality session, a CLI X11 capture confirmed Solaris was
selected on the Inner Sphere map. One click was sent to **Go To Planet**; the
screen stayed on the map and the trace added only the regular `0xa3` and `0xa7`
traffic. The session contains one earlier `0x91` before the initial `0xa8`,
but no later `0x91`, `0xad`, or `0x97`. The capture is
[`solaris-map-after-click-20261020.png`](../../../../MPBT3025/client/.runtime/captures/solaris-map-after-click-20261020.png).
The original server and game PIDs later exited; they were not restarted.

### Restarted live Solaris flow and repeatable join probe (2026-10-20, 10:24–10:29 local)

After the user authorized starting the services if needed, the server and client
were started with the commands above. The game opened on the intro screen; a
CLI X11 capture showed it clearly after the first early capture was black. The
visible Next button advanced to the Inner Sphere map and the call-sign form.
Using the existing test pilot `Subvoltic` produced `0x87 CREATE_CHARACTER`;
the server answered with `0x88` and `0x3c`. Exiting the training chooser
returned to the map with Solaris selected and **Go To Planet** active. The
Basic Training lesson itself was not launched; this live check covers the map
and join protocol path, not tutorial combat.

The fresh trace is
[`session-1792517080500724733.log`](../../runtime/traces/session-1792517080500724733.log).
It records the 2,073-byte 121-key `0xa8` after the first global `0xa7`, followed
by a single `0x91 JOIN_PLANET` for Solaris (planet 121) after the button click.
The screen stayed on the Inner Sphere map but changed to show the
Communications and **MechWarriors on Planet** panels. The trace tail after the
join contains five `0xa7` requests for planet 0 and three `0xa3` heartbeats;
neither `0xad` nor `0x97` appeared. The probe therefore reaches the server-side
join request and changes the map panels, but has not yet reached a lobby-room
request or left the map.

The key captures are
[`solaris-after-go-to-planet-20261020.png`](../../../../MPBT3025/client/.runtime/captures/solaris-after-go-to-planet-20261020.png)
and
[`solaris-after-wait-20261020.png`](../../../../MPBT3025/client/.runtime/captures/solaris-after-wait-20261020.png).
The repeatable map-only driver is
`client/scripts/probe-solaris-join.sh --confirm-map`; it captures the screen and
trace around the Solaris selection and one **Go To Planet** click, then watches
the trace for up to 30 seconds for `0x91` and `0xad`. It requires
the explicit map confirmation because it sends UI input, and it does not start
or stop either process. `client/scripts/x11-game-input.c` provides its XTest
mouse click. No VCR planet-state flag or populated `0x97` response was used.

A read-only X11 capture at 10:41 local is
[`solaris-current-20261020.png`](../../../../MPBT3025/client/.runtime/captures/solaris-current-20261020.png).
It still shows the selected Solaris map with empty **Communications** and
**MechWarriors on Planet** panels. The prior 10:35 capture had 22 global `0xa7`
requests and 11 `0xa3` heartbeats; by this 10:41 capture, the trace had grown
to 34 `0xa7` requests and 17 `0xa3` heartbeats, still with no `0xad` or `0x97`.
The map's bottom status ticker changed between captures (including a
`House Steiner: 2776` line), while the room panels stayed empty. The driver now
refuses to send another `0x91` if the active trace already contains a join, and
it checks a cropped footer fingerprint from the known pre-join map before
clicking. The current joined screen and the training chooser both fail that
screen check.

Static cross-check of the game-PDU factory confirms the room-request opcode:
factory case `0x2c` constructs the class at `BT3025.exe:0x521ed0`, whose class
descriptor is `0x593ccc`. The descriptor's first word is `0x00ad`; the factory
case number is an internal class ordinal, not the wire ID. Keep
`REQUEST_LOBBY_ROOMS` at `0xad` in the server.

`client/scripts/start-solaris-test-stack.sh` now brings up only missing test
processes and waits for the visible game window. Its reuse path was run against
the current stack and confirmed the existing server and game PIDs stayed the
same. A newly started server uses only the previously observed 121-key numeric
probe; it does not enable VCR state or send a ready-room list without a request.

### Delayed client fault and automated startup-to-join repeat (2026-10-20)

The earlier live client later showed a Wine **Program Error** dialog. Its log,
`client/.runtime/logs/game-20261020-102421-19048.log`, reports an unhandled
write fault at `BT3025.exe:0x5388fd` (`bt3025+0x1388fd`) to address `0x00000004`.
Wine's detail window records `ECX=0` and the faulting `movl %edi,0x4(%ecx)`;
the backtrace enters the nearby client heap bookkeeping/deallocation code at
`0x53884b`. This establishes a client heap fault, not its cause. It happened
after the session's 2,073-byte 121-entry `0xa8` snapshot and prolonged map
traffic, so the snapshot remains a possible contributor but is not proven to
be responsible. Captures:
[`solaris-current-continue-20261020.png`](../../../../MPBT3025/client/.runtime/captures/solaris-current-continue-20261020.png)
and
[`solaris-client-crash-details-20261020.png`](../../../../MPBT3025/client/.runtime/captures/solaris-client-crash-details-20261020.png).

After the crash, the Wine session and client were reset while server PID 19041
was left running. A fresh client connection used the same full numeric probe.
Trace
[`session-1792518726106206771.log`](../../runtime/traces/session-1792518726106206771.log)
records `0x87 CREATE_CHARACTER`, the server's `0x88`/`0x3c` pilot update, one
2,073-byte `0xa8` snapshot (2,067-byte PDU body), and then `0x91 JOIN_PLANET`
for Solaris (121). The map drew its planet symbols and Solaris details; after
the join click it showed **Communications** and **MechWarriors on Planet**,
both empty. The 30-second post-click watch found a heartbeat and another global
`0xa7` only. There was no `0xad REQUEST_LOBBY_ROOMS`, no `0x97 READY_ROOM_LIST`,
and no visible lobby transition. A later 10:59 capture, about six minutes after
the click, still showed the map; the trace had seven `0xa3` heartbeats and 14
global `0xa7` polls, with no crash dialog. This longer repeat still does not
rule out the earlier delayed fault.

The repeat captures are
[`solaris-relaunch-game-20261020.png`](../../../../MPBT3025/client/.runtime/captures/solaris-relaunch-game-20261020.png),
[`solaris-after-exit-training-20261020.png`](../../../../MPBT3025/client/.runtime/captures/solaris-after-exit-training-20261020.png),
and
[`solaris-join-20261020-105349-after-wait.png`](../../../../MPBT3025/client/.runtime/captures/solaris-join-20261020-105349-after-wait.png).
The new
`client/scripts/run-solaris-join-test.sh --confirm-startup-and-join` wrapper
starts or reuses the stack, recognizes the intro, call-sign, training, and
pre-join map screens, enters/reuses `Subvoltic`, exits the lesson chooser, then
calls the guarded single-join probe. Screen fingerprints live in
`client/scripts/screen-templates/`; the verifier is
`client/scripts/verify-solaris-startup-screen.py`. The X11 input helper now
types ASCII letters and digits for the call-sign field. The screen verifier
matched each captured stage, and the helper successfully entered `Subvoltic`.
The combined wrapper was not run from a fresh intro after being added; it was
run against this joined session and correctly refused before sending any input.

`start-solaris-test-stack.sh` now waits for two consecutive visible 800x600
window checks, rejects a visible Wine Program Error dialog, and uses `setsid`
when it launches a missing process so the helper shell can exit independently.
Its reuse path was verified without changing server PID 19041 or game PID
23041. The original heap-fault session is evidence of a failure; neither this
repeat nor the automation establishes the `0xa8` element semantics or the
missing map-to-lobby response.

### SDS starting ownership seed and lobby evidence (2026-10-20)

The SDS Appendix A lists planets by initial House affiliation. Its map rules
say House/district borders come from that appendix and the planet marker/name
take the current owner's House color (§§4.5.2.1, 4.5.2.1.3); the selected
planet also shows its controlling House and a balance chart derived from hex
control (§§4.5.3.1.2, 4.5.3.1.7).

Matching Appendix A against the 120 non-Solaris names in
`internal/game/planet_names.json` yields 108 unique starting affiliations:
22 Davion, 22 Kurita, 18 Liao, 24 Marik, and 22 Steiner. Eleven cache names
are absent from Appendix A, while Winfield appears under both Davion and
Steiner. Those twelve entries remain unassigned; Solaris remains neutral as
the SDS explicitly requires. `DefaultWorld` now seeds the unique matches at
100% control for their listed House. New saved worlds record
`ownership_seed_version: 1`. On open, an old world migrates only if all 120
owners/control maps exactly match the previous round-robin prototype and no
battles are recorded; custom ownership and campaign history are preserved.

This establishes server-side initial owner data, but by itself does not color
the running client map. At the time of this note, the `0xa8` entry fields had
no verified owner/control mapping. The SDS is product behavior, not a packet
schema.

The lobby investigation remains unresolved. The SDS describes a city lobby as
a persistent location with a view, title, user list, description, and
navigation hotspots (§§4.1.6, 4.12.1), but it does not name the success PDU.
Current code persists `JOIN_PLANET` (`0x91`) into the Solaris city model; live
trace `session-1792518726106206771.log` still shows no post-join server PDU,
no client `0xad REQUEST_LOBBY_ROOMS`, and no visible transition. Keep avoiding
the VCR-derived `0xa8` and populated `0x97` experiments.

The implementation was formatted and compiled to `/tmp` with the bundled Go
toolchain; no Go tests were run. Existing server PID 19041 and game PID 23041
were left running. The saved world file and current live session were not
modified; the ownership migration applies on a future server startup.

### Automated startup-to-join repeat (2026-10-20, 11:25 local)

The first wrapper attempt launched a fresh client but read the previous
session's trace before Aries login had created the new one. After correcting
trace selection and screen matching, the wrapper reran against that fresh
client and completed the full path. It now waits for a new session trace when
it starts a missing client and safely reuses the current trace when the game
was already running. The startup screen matcher compares edge layout for the
intro button and other fixed UI regions, since the current Wine render has a
brightness shift from the saved templates; it still uses pixel color matching
for distinguishing the empty and `Subvoltic` call-sign fields.

The run recognized the intro, typed the `Subvoltic` call sign, submitted it,
exited the training chooser, verified the pre-join map, then clicked **Go To
Planet** once. Captures are
[`solaris-startup-20261020-112457-intro.png`](../../../../MPBT3025/client/.runtime/captures/solaris-startup-20261020-112457-intro.png),
[`solaris-startup-20261020-112500-call-sign-empty.png`](../../../../MPBT3025/client/.runtime/captures/solaris-startup-20261020-112500-call-sign-empty.png),
[`solaris-startup-20261020-112504-call-sign-subvoltic.png`](../../../../MPBT3025/client/.runtime/captures/solaris-startup-20261020-112504-call-sign-subvoltic.png),
[`solaris-startup-20261020-112507-training.png`](../../../../MPBT3025/client/.runtime/captures/solaris-startup-20261020-112507-training.png),
and [`solaris-startup-20261020-112509-map.png`](../../../../MPBT3025/client/.runtime/captures/solaris-startup-20261020-112509-map.png).

Trace
[`session-1792520472103837304.log`](../../runtime/traces/session-1792520472103837304.log)
records one 2,073-byte `0xa8` update with 121 numeric keys, `0x91` for
Solaris (121), ten global `0xa7` requests, and eight heartbeats. It contains no
`0xad` lobby-room request or `0x97` room list. The after-wait capture
[`solaris-join-20261020-112510-after-wait.png`](../../../../MPBT3025/client/.runtime/captures/solaris-join-20261020-112510-after-wait.png)
and a fresh 11:28 capture
[`solaris-current-20261020-1128.png`](../../../../MPBT3025/client/.runtime/captures/solaris-current-20261020-1128.png)
still show the Inner Sphere map with empty Communications and MechWarriors
panels. The client remained alive while sending repeated global state polls.
The House/district boundaries are colored, the inner map cells remain gray,
and neutral Solaris remains white. This confirms the automated join probe and
reproduces the lobby blocker; it does not validate live planet owner
projection. The 121-entry `0xa8` state fields are still all zero and unnamed.

The test did not restart server PID 19041. The missing game was started and is
running as PID 26888. The join updated the persisted pilot location at 11:25;
the ownership migration has not been applied to the running server's in-memory
world and remains for a future startup.

### Faction-colored territory reference and opt-in owner snapshot (2026-10-20)

The user supplied `p2t226-b-fresh-map.webp` as the expected fresh-map
appearance. It clearly shows the 120 Inner Sphere territories filled by their
owning House color: Steiner blue, Kurita red, Davion yellow, Marik magenta,
and Liao green. Solaris is neutral and white. This resolves the prior visual
uncertainty: the target includes faction-colored territory fills, not only
colored borders or planet markers. `p2t235-b-ready.webp` shows the Solaris
mission-room view and two cadets in the Alpha and Beta lances; it is a separate
later-stage reference and does not define the join wire response.

The local `BT3025.vcr` HouseInfo IDs are 0 `none`, then 1 Davion, 2 Kurita,
3 Liao, 4 Marik, and 5 Steiner. A first opt-in owner snapshot placed these
IDs in the second scalar of each `0xa8` element, but further disassembly
showed that scalar is used as the key to a map-object lookup at `0x479290`,
and again to update that object at `0x479230`/`0x478df0`. Substituting five
House IDs for 121 map-object keys would alias many territories. The unrun
owner-snapshot mode was removed. The first scalar is the collection lookup
key; the owner encoding is still unknown. The earlier all-zero 121-key probe
checked list cardinality only and cannot validate ownership colors.

The saved `world.json` still has no `ownership_seed_version` and retains the
prototype's 24-planets-per-House round-robin assignment. Server PID 19041 has
that same older world in memory. On a future startup, the existing conservative
migration should replace only that untouched synthetic seed with the SDS
starting owners; the opt-in owner snapshot then reads the migrated world. The
live world file was not edited while the server was running.

The first join milestone remains open. The current trace still ends with the
client on the Inner Sphere map after `0x91 JOIN_PLANET` for Solaris, without a
visible transition or `0xad REQUEST_LOBBY_ROOMS`. Existing server PID 19041
and game PID 26888 were left running while the owner-snapshot code was added.

### Full client-cache owner seed and repeat join check (2026-10-20, 11:47–11:52)

The prior game process later showed a Wine page fault at `BT3025.exe:0x5388fd`
after roughly 24 minutes of map polling. Its trace ended at 11:44:41 and the
client log recorded the fault at 11:45:02. The fault is in a pointer write and
does not identify the preceding game packet as its cause. The crashed client
and Wine debugger were closed; the existing server PID 19041 was preserved.
The guarded Solaris UI script launched a fresh client, completed the known
intro/call-sign/training path, selected Solaris, and clicked **Go To Planet**
once. Trace `session-1792522066610043032.log` records `0x91` planet 121,
one 121-entry `0xa8` reply after `0xa7`, and no `0xad` within the 30-second
watch. Capture `client/.runtime/captures/solaris-join-20261020-114804-before.png`
has black territory interiors; the `after-wait.png` capture has gray cells and
white interior borders, still without a lobby transition.

Closer inspection of the preserved `PlanetInfo` VCR records found their
serialized `m_house` field. The `PlanetInfo` formatter names the field at
`BT3025.exe:0x515f56`, and the serializer emits it after ID, name, and type
at `0x515cc9`–`0x515cd3`. Matching all 121 `[ID, name]` prefixes in the
hash-checked VCR gives 25 Davion, 25 Kurita, 20 Liao, 25 Marik, 25 Steiner,
and Solaris with special value 7. This resolves the twelve previously
unassigned SDS names. It also differs from the SDS-derived assignment for
Dixie (ID 83): the client cache says Steiner. The client cache is now the
initial owner source in `internal/game/planet_names.json` for all 120
non-Solaris worlds. Solaris remains neutral in the server model.

New worlds have `ownership_seed_version: 2`. On load, version 0 synthetic
round-robin or version 1 SDS ownership migrates only if the owner/control
data still matches that exact untouched seed and no battles have been
recorded. The previous SDS seed is retained in
`internal/game/planet_sds_owners_v1.json` only to recognize it for safe
migration. The current live `world.json` remains version 0, and the running
server still has its old state in memory. Go package checks for game,
protocol, and server pass; new game checks cover owner counts, Dixie, and
preservation of edited ownership. The map wire owner field is still unknown,
so this change alone does not color the client screenshot.

### House-colored map validated; Solaris join still pending (2026-10-20, 11:53–12:00)

The earlier inference that the second `0xa8` scalar was a planet-object lookup
key was wrong. A controlled A/B visual trial on isolated listeners resolved
it. The 121-entry ID-mirror probe (`Key=planet ID`, second scalar=planet ID)
colored only a small cluster near the map center; capture:
`client/.runtime/captures/id-mirror-before.png`, trace:
`runtime/traces/id-mirror/session-1792522423254859330.log`.
The House probe (`Key=planet ID`, second scalar=owner HouseInfo ID) colored all
five territories as in the user's reference image; capture:
`client/.runtime/captures/house-probe-after-a8.png`, trace:
`runtime/traces/house-probe/session-1792522545488433674.log`.
Davion=1/yellow, Kurita=2/red, Liao=3/green, Marik=4/magenta,
Steiner=5/blue, and Solaris=0/neutral in this state PDU. The client stayed
stable. The ID-mirror result and House result show this scalar drives the
faction-colored territory fills. Both isolated clients sent `0x91` when
Solaris was clicked, but neither entered the lobby.

The normal server path now builds a 121-entry owner snapshot from its live
campaign world in response to the first global `0xa7` poll. It sends another
only when the owner values change, incrementing the snapshot sequence. The
unconfirmed VCR and schema probes remain explicitly opt-in. A fresh default
server on port 15004 and client confirmed the normal path: trace
`runtime/traces/default-owner/session-1792522732550236331.log`
contains `0xa7` followed by a 2,073-byte `0xa8`; screenshot
`client/.runtime/captures/default-owner-after-a8.png` passes all five region
checks in `client/scripts/verify-faction-map-screen.py`. The prior gray map
capture fails those same checks. After one Solaris click, the client sent
`0x91` for ID 121, continued polling `0xa7`, sent no `0xad` during 30 seconds,
and stayed on the correctly colored map in
`client/.runtime/captures/default-owner-after-join.png`.

Go tests pass for game, protocol, and server packages, including owner counts,
legacy seed migrations, and owner-snapshot wire values. The original server
PID 19041 on port 15000 was not restarted and still has its old in-memory
world/probe. The validated default server PID 36313 and game PID 36346 remain
running on port 15004. The two temporary experiment listeners on ports 15002
and 15003 were stopped. The next work is the first successful Solaris lobby
transition after `0x91`; the screenshot and trace do not show it yet.

### Login identity and planet presence check (2026-10-20)

The normal port-15004 trace begins with a 334-byte Aries `LOGIN` frame whose
two account-name slots read `unknown`; the launch key is `local-dev`. The
client later sends `0x87 CREATE_CHARACTER` with the nine-byte call sign
`Subvoltic`. The server replies with `0x88` and a call-sign-specific `0x3c
PLAYER_UPDATE`. It had already sent placeholder `Local Pilot` records for
both `0x3c PLAYER_UPDATE` and `0x3d PROFILE_UPDATE` before the character
request. It sends no matching `0x3d` refresh afterward. After `0x91
JOIN_PLANET`, the server persists the Solaris location without a room or
occupant update.

The live capture `client/.runtime/captures/current-login-presence.png` shows
the faction-colored map, Solaris selected, and an empty **MechWarriors on
Planet** panel with count 0. The trace contains no `0xad REQUEST_LOBBY_ROOMS`
or observed occupant request after the join. This supports the hypothesis
that session identity/presence or the successful location handoff is missing;
it does not demonstrate an extra password login requirement. A previous
isolated `0x3c` refresh after the join did not change the screen, and a
`0x3d` refresh with `m_lastPlanet` set to Solaris terminated the client.
Recover the actual room/occupant PDU layout and receive path before another
live transition payload experiment.

The 2001 Player's Guide says the player first signed into EA.com with a member
name and password, then clicked **Play** to launch the game (manual page 5).
The current test invocation starts `BT3025.exe` directly with
`aol:127.0.0.1:15004:key:local-dev`; it skips that retired account sign-in and
the old EA/GILS `STARTGAME` bootstrap. A preserved GILS record confirms that
the old handoff carried separate account/handle/session fields alongside the
game connection string; its credential values are intentionally not copied
into this note. Aries has startup argument parsing for `email`, `user`, and
`password`, and its defaults include `unknown`, matching the direct-launch
trace. This explains the `unknown` account label without implying that the
call-sign form was an account login.

Keep the identities distinct: the account/member name authenticated the EA
session upstream; the call sign names the in-game pilot and is what the
planet-presence list is expected to show. The direct local launcher has no
real EA account authentication, but the empty planet list remains a separate
missing room/occupant or world-location handoff after the pilot's `0x91` join.
See `client/docs/linux-client-setup.md` for the operator-facing explanation.

### Solaris planet presence update validated (2026-10-20, 12:24)

Disassembly of the `OCCUPANT_UPDATE_PDU` metadata at `0x5941c8` identifies
message ID `0x94`. Its serializer at `0x529b30` writes the nested
`BTPlayerInfo` object with the same serializer used by `PLAYER_UPDATE` (`0x3c`).
The local server now sends this call-sign-specific record immediately after a
successful `JOIN_PLANET` (`0x91`). The Go integration test checks that `0x94`
uses that recovered player record, and `go test ./...` passes.

A fresh client on isolated port 15005 selected Solaris and clicked **Go To
Planet** once. The trace
`runtime/traces/occupant-probe/session-1792524246412071876.log`
records one received `0x91 JOIN_PLANET` and one sent `0x94 OCCUPANT_UPDATE`
(41-byte record body). The screen then showed **MechWarriors on Planet: 1** and
**Cadet Subvoltic (Lvl 0)**; capture:
`client/.runtime/captures/occupant-probe-after-join-20261020-1224.png`.
The map stayed visible, as in the user's fresh-map reference. The client made
no `0xad REQUEST_LOBBY_ROOMS` request during the following 30 seconds, and the
communications pane stayed empty. This confirms planet presence and resolves
the empty roster for a local join; the room-list/chat handoff remains a
separate step. The earlier login fields still read `unknown` on this direct
local launch, but that did not prevent the client from showing its callsign.

The test scripts now select the unique 800x600 game window by its dimensions,
so stale 160x31 title windows from earlier Wine sessions do not block capture
or the guarded join probe.

### Solaris chat identity and channel routing (2026-10-20, 13:50–14:05)

Two isolated clients on port 15008 sent planet chat after joining Solaris.
Both used message ID `0x08`; the PDU body carries the sender call sign (the
BetaPilot packet includes the length-prefixed string `BetaPilot`) and ends in
a channel byte. Live captures show All as `0x00` and House as `0x02`. All chat
worked in both directions between Subvoltic and BetaPilot. A House message
also reached the other pilot under the original planet-wide relay, even though
neither pilot had a House affiliation.

On the map, Tab cycles between All and House and wraps back to All. The manual
restricts Team to Mission Rooms and combat, so no live Team message could be
sent before the missing lobby/room transition. The chat PDU serializer writes
its channel as the final byte; with observed All=`0x00` and House=`0x02`, the
server now treats Team=`0x01` as the intervening enum value. This is a static
inference, not a captured Team packet.

The server relay now uses each joined pilot's persisted House and location:
All stays within the same city, sector, or room, and House is delivered only
to pilots with the same non-empty House in that location. Unrecognized channel
values are echoed only to the sender until their routing context is known.
Team is routed only to seated pilots with the same room ID and lance side;
observers in that room can receive All but not Team. Unit tests cover all three
channels, location boundaries, unassigned pilots, observers, and the sender-only
fallback. This routing is not reachable from the live client yet because the
room-join and seat-update message path is still unresolved. `go test ./...`
passes.

A live check of the modified build was attempted on isolated port 15009, but
the fresh Wine prefixes reopened the client introduction flow and did not
reach chat. That run does not count as live verification of the new House
filter. The original game and server were left untouched; the successful live
All/House observations above are from the prior build before filtering.

### Solaris guide rules represented in the world model (2026-10-20)

The game model now represents the guide's Solaris progression and return-city
flow in `internal/game`: cadets enter a capacity-checked city, graduates return
to a sixteen-city directory and must select a city, occupancy is measured in
'Mechs; Cancel restores the location from before the picker. City capacity is
persisted as `max_mechs` while still reading old
`max_pilots` saves. Each city maintains available empty practice rooms for
cadets and graduates separately. Room rules allow either side, block mixing
across graduation status, start a sixty-second countdown, run a lone ready
pilot against an AI opponent, rebalance same-side ready players, and replenish
empty room choices when one launches or completes. Solaris results award rank
to cadets, grant the Graduation Medal at 1,200 points, and do not add rank or
C-bills to graduate return battles. The current rank award values remain
prototype scoring because the guide defers detailed scoring to another section.

Rejected world updates now restore their pre-update in-memory state. Leaving a
room preserves the existing countdown while at least one other pilot remains
ready. Regression tests cover both cases, the All/House/Team chat routing model,
room observers, and same-lance filtering. Standard and race-instrumented Go test
runs pass, and the server builds.

These are persisted game rules and are covered by `go test ./...`; they do not
complete the legacy client's UI path. The map-to-city success update remains
unidentified, `0xad` has not appeared in the observed map flow, and a populated
`0x97` room list is still intentionally withheld because its fields' meanings
are not established. The request/response codecs and state model should be
connected only after the client receive path is understood; the earlier invalid
populated-list experiment crashed the client.

The return-city action now requires a graduated pilot to be in the pending
Solaris city-picker state created by `JOIN_PLANET`; cadets remain in their
automatically assigned city and cannot invoke the graduate-only picker. This
prevents out-of-order city requests from bypassing the manual flow. Regression
coverage also checks that rejected city changes leave the saved location intact.
The full Go suite passes after this guard. No live client/server process was
restarted, and the unavailable wire transition remains the blocker for
end-to-end lobby testing.

For a controlled live trial only, the server also has the opt-in
`-experimental-solaris-empty-room-list-after-join` switch. It sends the verified
empty two-container `0x97` after the normal occupant arrival following a
Solaris join. The default remains off; no room entries are fabricated. The
bootstrap integration test checks this exact message order. A client trial is
still required to learn whether pairing the empty list with occupant presence
changes the screen.

### Empty room-list sequencing trial (2026-10-20, 15:15)

An isolated server on `127.0.0.1:15010` used a fresh world and the opt-in
empty-list switch; no VCR or planet-state probe flags were enabled. The client
reached the verified pre-join map with faction colors passing
`verify-faction-map-screen.py`, then received exactly one **Go To Planet** click.
Trace `runtime/traces/empty-room-after-presence/session-1792534557207857162.log`
records `JOIN_PLANET` (`0x91`), an `OCCUPANT_ARRIVED` (`0x2b`) carrying the
call-sign-specific `BTPlayerInfo`, then an empty `READY_ROOM_LIST` (`0x97`,
eight-byte body). After 25 seconds the client remained stable on the map,
showed one pilot in **MechWarriors on Planet**, and had sent no
`REQUEST_LOBBY_ROOMS` (`0xad`). The result matches the earlier empty-list-only
trial: this valid empty response does not produce the Mission List or the
city-lobby screen. Captures are
`client/.runtime/captures/solaris-empty-room-list-after-join-click.png` and
`client/.runtime/captures/solaris-empty-room-list-after-wait.png`.

The temporary client and server were isolated from the existing listeners;
the map transition remains unresolved. This narrows the next protocol search:
planet presence plus an empty ready-room list is insufficient, so the missing
success state has to precede these lobby contents or use another verified
message path.

While that isolated client remained on the map, live chat was also exercised.
The All message `testall` and House message `testhouse` both appeared in the
Communications pane and echoed through the server. The raw `0x08` bodies
include callsign `Subvoltic` and end with the observed channel bytes `0x00`
(All) and `0x02` (House). Captures:
`client/.runtime/captures/solaris-chat-all-live-check.png` and
`client/.runtime/captures/solaris-chat-house-live-check.png`. Team remains
unavailable to exercise in the live UI because the manual exposes it only in
Mission Rooms/combat, and the client has not reached a Mission Room; the model
test covers the inferred `0x01` route.

The guarded startup helper was adjusted after this trial: a missing test server
now starts with the validated default owner snapshot and gets a separate world
file on non-default ports. It refuses experimental planet-state/redirect
servers and checks that a reused game PID still has an established socket to
the selected server. `MPBT_SOLARIS_TEST_PORT` selects an isolated port, while
`MPBT_SOLARIS_TRACE_DIR` and `MPBT_SOLARIS_WORLD_PATH` can match a pre-existing
test server. Shell syntax checks pass; live checks confirmed it refuses the
existing 15000 full-probe server without stopping it and refuses a stale,
disconnected client window on 15010.

### GILS account and server connection evidence (2026-10-20)

The shipped GILS files provide direct evidence for the old EA connection path.
In `client/MPBT3025andGILS/GILS/gilslog.txt`, the client requests
`www.ea.com:10080/EAPlay`, receives an `application/x-gils-plugin` response,
and parses `STARTGAME` for game ID 2088. The `cmdtxt` launches the game with an
`aol:<server>:15000:key:<opaque-ticket>` string. GILS separately has the EA
member handle, account ID, session ID, and instance ID, and sends HTTP
`/GameHeartbeat` updates at two-minute intervals plus `GAMEQUIT` when exiting.
Values from the historical account and ticket are deliberately omitted here.

The failure capture `client/MPBT3025andGILS/GILS/gilslog2.txt` gets an HTML
response with “User ticket data not found,” then exits without a `STARTGAME`
record. Combined with manual page 5, which puts member-name/password login on
EA.com before **Play**, this confirms that GILS connected the authenticated web
flow to the game but was not itself the password-entry screen. It also shows
that EA's web launch service and the separate Aries game connection were two
parts of startup; the EA HTTP heartbeat is not a Solaris presence message.

An isolated local test on port 15011 appended `user=ProbeIdentity` after the
direct `aol:` connection string. Aries still sent both account-name slots as
`unknown`; the extra text was folded into the trailing connection argument.
This is not a valid way to set the account identity. The test client received
the ordinary bootstrap/configuration exchange and sent no Solaris join; that
test server and client were stopped afterward. The original listeners and game
were left untouched. A local account/handle mapping may still be useful for
our emulator, but the historical logs do not disclose the ticket validation
or identity-claim wire format, so the `local-dev` path must not be presented as
EA authentication.

### Callsign-safe two-client chat relay (2026-10-20)

The `0x08 BTCHAT_MSG_PDU` serializer writes its sender identifier as a
length-prefixed string at body offset 7, then writes `m_chatType` as the final
byte (`BT3025.exe:0x510280` and `0x529fe0`). The new protocol fixture matches
the live Subvoltic `test`/All packet byte-for-byte. The server now replaces
that sender field with the callsign associated with the connected pilot before
echoing or relaying the message, so a client cannot make the other player's
Communications pane attribute a message to a different callsign. The message
text and channel byte are preserved.

The two-client server integration now uses client-shaped chat PDUs. It joins
AlphaPilot and BetaPilot to Solaris, checks both clients receive the other
pilot's call sign, verifies All in both directions, verifies the server
corrects a spoofed sender field, and checks that unassigned cadets do not
receive House chat. Separate routing tests cover same-House delivery and
Team chat within one mission-room lance, while excluding opposing seats,
observers, and other rooms. The latter remains a model-level test because the
legacy client still never leaves the map. The integration test also reads both
chat recipients concurrently to avoid ordering deadlocks in its in-memory
transport. `go test -count=1 ./...` passes.

`scripts/summarize-trace.py` now labels `0x08` events and reports the
embedded sender callsign and channel. Running it against both preserved
two-client chat traces shows BetaPilot's live All and House packets and confirms
neither session sent `0xad` or received `0x97` during those map-only tests.

This verifies server-side two-session presence and chat routing, not a live
client Mission Room. The latest live client evidence remains the 15:15 map
trial above: the call-sign roster updated, All and House displayed, but no
`0xad REQUEST_LOBBY_ROOMS` followed the Solaris `0x91 JOIN_PLANET`. Do not
claim that two clients have entered a city or mission lobby until a live screen
transition is captured.

### Two-client roster order, live chat, and Solaris 1.23 reference (2026-10-21)

An isolated server on port 15011 ran without VCR or planet-state probe flags.
Subvoltic and BetaPilot connected from separate Wine prefixes so both 800x600
game windows remained visible. Each joined Solaris once. Both screens stayed
on the map and showed only the other pilot in **MechWarriors on Planet** (count
1). The second-join traces show the server sent each joining client the
existing pilot's `0x3c PLAYER_UPDATE`, then `0x94 OCCUPANT_UPDATE`, then its own
`0x2b OCCUPANT_ARRIVED`. The existing client received the new peer's `0x3c`
and `0x94`, followed by an experimental repeat of its own `0x2b`. That final
self-arrival did not restore the self row. A second trial sent an extra `0x94`
for the local pilot after the peer update; it also left the map roster at one
peer. That self-update experiment was removed as well. The earlier all-`0x2b`
trial also failed to add the peer. Do not treat these singleton updates as a
complete planet roster; the correct multi-entry map-roster message remains
unknown. Neither client sent `0xad` or received `0x97`.

The settled map captures show the territory fills in faction colors on both
clients. The black fill seen in an earlier immediate post-join capture did not
recur in this run.

Inspection of both captures confirms the **MechWarriors on Planet** count is
one on each client and the single row is the other pilot; neither local pilot
appears in its own list. The earlier extra-self `0x94` run also showed only the
peer after a peer update. The stable traces do include each joining pilot's
own `0x2b OCCUPANT_ARRIVED`, so replaying that same singleton message is not a
complete roster refresh. The default `0xa8` snapshot in this run had a 2,067-byte
body with 121 records, generated from the server's seeded world state; this
was not a VCR-derived payload or a planet-state probe flag.

The preserved Player's Guide says that a cadet who clicks **Go To Planet**
after Pilot Training should appear in a Solaris city lobby (pages 9–10). The
two live screens remained on the Inner Sphere map instead. This distinguishes
the missing city-lobby transition from the later strategic-sector and staging
area flows. The 1996 Solaris 1.23 GitHub reference remains useful for its
separate in-room combat protocol, but provides no BT3025 city-lobby handshake.

Live chat was exercised from both clients. All (`0x00`) appeared on both
Communications panes, attributed to `BetaPilot` and `Subvoltic` respectively.
House (`0x02`) messages from unassigned cadets appeared only for the sender,
matching the server's same-nonempty-House filter. Team is unavailable on this
map screen; its routing remains model-level until a Mission Room can be
reached. Captures are in `client/.runtime/captures/` with the names
`solaris-beta-chat-all-20261021.png`,
`solaris-subvoltic-chat-reply-20261021.png`,
`solaris-beta-chat-house-20261021.png`, and
`solaris-subvoltic-chat-house-20261021.png`.

The requested earlier-version reference is cloned at
`references/solaris-protocol-reference/` from
[`RazorWing/solaris`](https://github.com/RazorWing/solaris), revision
`bcf5913d518af7ca604535ff8572d280350c86e0`. Its README identifies the target
as Multiplayer BattleTech: Solaris 1.23 and calls the server a packet/command
prototype. The Python server documents a COMMEG32 `0x1e` handshake and in-room
combat commands such as room initialization; it does not describe the 3025
Aries shell's planet join or `0xad` lobby handoff. The clone has no license
file, so keep it as a local protocol reference and do not copy its code into
the emulator without resolving reuse terms.

The two-player occupant integration assertions now match the packets that
remain after removing the ineffective self-refresh. `gofmt` and
`go test -count=1 ./...` pass. No populated `0x97` was sent.

The trace summarizer now labels the generic occupant-arrived/departed and
update-occupants messages, and extracts the call sign from recovered
`BTPlayerInfo` records in `0x2b`, `0x2c`, `0x3c`, and `0x94`. It was syntax
checked and run over both preserved stable traces; it shows the joiner's own
`0x2b` and the peer's `0x94` by call sign, while neither trace contains
`0xad`.

`client/scripts/capture-test-evidence.sh` now accepts an optional fourth
argument containing a specific 800x600 BT3025 window ID. This keeps the
single-window default safe while allowing separate evidence captures when two
clients are open. The current visible windows were captured without input or
restart as `client/.runtime/captures/solaris-current-beta-20261021.png` and
`client/.runtime/captures/solaris-current-subvoltic-20261021.png`. Both are
still on the map, show the other pilot only (count 1), and have no `0xad` or
`0x97` in their associated traces.

### Nonzero player-ID trial and pause handoff (2026-10-21)

An isolated server on port 15011 was used to test whether the missing self row
was caused by the current zero-valued `m_playerID`. No VCR planet-state flags
were used, and no populated `0x97` was sent. The two trace files are
`runtime/traces/test-15011-occupant-only/session-1792606831417930696.log`
and
`runtime/traces/test-15011-occupant-only/session-1792607012689036049.log`.
They show Solaris `0x91` joins followed by `0x94` and `0x2b` records carrying
`m_playerID=1` for Subvoltic and `m_playerID=2` for BetaPilot. The map roster
still showed zero players. BetaPilot's client displayed a Microsoft Visual C++
runtime error for abnormal program termination; the first client process also
ended after its join. Captures are
`client/.runtime/captures/occupant-only-a-after-join.png`,
`client/.runtime/captures/occupant-only-b-after-join.png`, and
`client/.runtime/captures/occupant-only-b-runtime-dialog.png`. The two traces
contain neither `0xad REQUEST_LOBBY_ROOMS` nor `0x97 READY_ROOM_LIST`.

That result rules out sending arbitrary locally assigned IDs in these
singleton occupant records. The code and integration expectations have been
returned to the last non-crashing format: outbound player/occupant records use
ID 0, and a planet join sends the joining client's `0x94`, existing peers'
`0x3c` and `0x94`, then the joining client's `0x2b`; connected peers receive
the new client's `0x3c` and `0x94`. The `0x88` character-creation reply remains
the only immediate post-create response; the extra `0x3c`/`0x3d` refresh is
still omitted. Persistent pilot IDs and the `WithID` serialization helpers
remain in the local model/code, but must not be put on the wire until a real
account/player identity and the client's accepted roster semantics are known.
The related bootstrap and two-client sequence tests were realigned with this
format. `gofmt` and `go test -count=1 ./...` pass.

The ordinary world-derived `0xa8` owner snapshot continues to color the map's
territory fills by faction. The unresolved blockers are the Aries/GILS account
identity context and the client-accepted way to send a complete planet roster
and trigger the city-lobby transition. Direct `aol:` startup still identifies
the account as `unknown`; guessed numeric IDs crash this client, and the
singleton `0x94`/`0x2b` join sequence leaves it on the map. Do not claim a
two-player lobby or battle is reachable yet. Next work should inspect the
client's account-name/identity path and decode a roster/transition PDU from
authoritative client evidence before another live experiment. Keep testing on
an isolated listener with at most the two existing client prefixes; do not
retry guessed IDs, VCR state, or a populated `0x97`.

The pause-turn source edits are in `src/server/main.go` and
`src/server/main_test.go`; the handoff updates are in this document.
Earlier implementation and evidence (faction-colored map ownership, callsign
chat relay/channel routing, trace decoding, and the Solaris 1.23 reference)
are documented in the preceding sections above.

The isolated 15011 server and its test clients were stopped for this pause.
No other listener or game process was restarted as part of the cleanup.

### String PlayerID recovery and room receive path (2026-10-21, resumed)

The earlier `WithID(..., uint32)` implementation was incorrect. The base
player serializer at `0x4fce40` sends the object at `+0x04` through the virtual
object writer `0x4f7770`. That object's vtable is `0x5914e4`; its writer
`0x4f7ff0` calls the string writer `0x4f7710`, and decoder `0x4f8010` calls
the string decoder `0x4ed520`. The debug formatter at `0x4f8030` labels this
string `m_ariesID`. Thus the former four zero bytes were an empty string
length. Writing numeric 1 or 2 there made the client consume unrelated bytes
as identity text and misaligned the rest of the record. This explains the
runtime failures without requiring real EA credentials.

The `0x94` receive dispatch at `0x4de77b` calls `0x500290` to remove the old
occupant by PlayerID and `0x500150` to insert the replacement. Sending the same
empty identity for all pilots therefore replaced the previous row. In
contrast, `0x3c` handler `0x4df150` calls `0x4f4000`, which replaces the
receiving client's own player record. The old peer `0x3c` broadcasts were
incorrect even though their callsigns appeared to fix the visible peer row.

`internal/protocol/pilot.go` now writes string IDs and uses each pilot's
callsign as a local identity for `For` helpers, keeping the bootstrap ID empty.
The explicit `WithID` helpers accept strings. `cmd/server/main.go` broadcasts
only occupant messages for peers and does not replace the local player with
peer data. The numeric persisted IDs remain internal database identifiers.
Protocol regression tests check the actual string length/data and following
callsign alignment; the two-client integration and hub assertions forbid peer
player updates. The full Go test suite passes. The trace summarizer now parses
variable-length PlayerID strings before reading the callsign.

On isolated port 15011, the corrected server sent two distinct `0x94` records
to BetaPilot and its own `0x2b`. A live capture shows count 2 and both
**Cadet BetaPilot** and **Cadet Subvoltic**:
`client/.runtime/captures/string-id-b-two-players.png`. The trace is
`runtime/traces/string-player-id/session-1792608043213374200.log`;
Subvoltic's associated trace is `session-1792607938701147789.log`. There is
still no `0xad` request or lobby transition. Before the reciprocal screenshot,
the server and both client command sessions terminated together with exit
status 143; the cause is not established and no runtime-error dialog was
observed. A repeat uses detached processes and the existing two prefixes.

The actual room-change receive path is also identified: generic wire dispatch
`0x4f3cf0` maps `0x3e UPDATE_ROOM` through virtual slot `+0x14` to
`0x4df1d0`, which calls `0x4f3f60` to replace the current room, store the
reason code at connection `+0x13c`, optionally clear occupants, and emit event
9. Descriptor `0x592d60`, encoder `0x505f90`, and decoder `0x505f50` confirm
the body is uint32 reason, bool occupant-list-follows, then nested BTRoomInfo.
RoomID is itself a composite object: vtable `0x591508`, writer `0x4f8710`
emits uint32 room number followed by a host-name string; decoder `0x4f8740`
confirms both. BTRoomInfo's planet at `+0xa4` is a numeric wrapper whose value
is `+0xa8`, not a string. The complete room serializer is `0x5018b0` plus
`0x51f1f0`. These types must be respected before any room-update experiment.

The detached repeat confirms the roster fix in both directions. Captures
`client/.runtime/captures/string-id-detached-a-two-players.png` and
`string-id-detached-b-two-players.png` each show count 2 and both callsigns.
Stopping only BetaPilot's prefix caused a string-ID `0x2c` departure; the
remaining Subvoltic client correctly retained its self row and changed to
count 1 (`string-id-detached-a-peer-left.png`). Traces are under
`runtime/traces/string-player-id-detached/`. The new helper
`client/scripts/advance-solaris-client.py` advances a specified existing game
window and callsign through recognized startup screens, clicks Go To Planet
once, and waits for `0x91` in the selected trace. It supports either of the
two existing client windows and never launches additional clients.

The room-update type-1 trial succeeded. Isolated server command:
`runtime/mpbt3025-room-update -listen 127.0.0.1:15011 -world runtime/world-test-15011-string-id.json -trace-dir runtime/traces/room-update-type1 -experimental-planet-room-type=1`.
Trace `runtime/traces/room-update-type1/session-1792610558991047031.log`
shows the Solaris `0x91`, then an 89-byte `0x3e` body (reason 1, occupant list
follows true, RoomID number 121001/host local, name Solaris, room type 1,
planet 121), then string-ID `0x94` and `0x2b`. The screen changed to
**Staging Area / Missions on Solaris** and Communications says **You are on
Solaris**. Capture: `client/.runtime/captures/room-update-type1-a-live.png`.
The client stayed connected and sent its heartbeat. The mission list is empty;
no `0xad` or `0x97` appeared. An authenticated EA login is therefore not needed
to produce this local lobby transition.

The room-list cache receive handler `0x4dfd00` handles generic `0x27` and
merges full RoomInfo records, then emits event 15. Its serializer at
`0x509f70` writes result uint32, reason string, count, then each RoomInfo.
The next controlled trial supplies the existing city model's cadet missions
through this recovered generic list. It sends no populated `0x97`.

### Staging missions and first mission join (2026-10-21)

Generic 0x27 is accepted but does not populate the staging mission cache.
The staging view reads connection +0x1c0. `0x99 READY_ROOM_ADDED` inserts into
that cache: full BTRoomInfo followed by a counted CombatSeatLite map
(writer 0x4e65d0, decoder 0x4e66d0). The decoder constructs **CombatSeatLite**
at 0x516880, not full CombatSeatInfo. Each lite record is u8 seat, u8 team,
string callsign, u32 house, u32 rank, u8 badge (writer 0x516920).

Live proof: `client/.runtime/captures/room-added-a-after-join.png` displays
Cadet Mission 1–3 with 0/4 on each lance and an enabled Enter Mission button.
Trace: `runtime/traces/room-added/session-1792611086469906261.log`.
Clicking Enter Mission sent 0x23 JOIN_ROOM, body
`00000001000000056c6f63616c00000000`: room number 1, host "local", empty password.
No populated 0x97 was sent.

Implemented JOIN_ROOM parser and atomic Store.JoinAvailableRoom, assigning the
least populated lance. The first live join succeeded in world state and the
native screen switched to **Mission Room**, Communications "You are in Cadet
Mission 1", roster "MechWarriors in Mission" with Subvoltic. Screenshot
`client/.runtime/captures/mission-join-a-stable.png`; trace directory
`runtime/traces/mission-join/`, latest session 1792611614517017136.
Own 0x3c with a correctly encoded string PlayerID is accepted here; earlier
numeric-ID crashes were malformed string lengths, not proof of missing EA login.
The initial mission screen has no lance rows until CombatInfo is supplied.

Work in progress: `0x9e CURRENT_COMBAT` serializes CombatInfo (0x514ff0),
composite RoomID, u32 planet/map/climate/terrainSeed, u8 mech visibility,
bool sanctioned, bool mechVisible, then counted full CombatSeatInfo map
(0x42e890/0x42e990). Full seat writer 0x517150 adds weightClass, string
occupant PlayerID, u32 MechID, state byte, score fields to the lite base.
Codec implemented; live validation pending. Seat enum and valid mech/map IDs
still need checking. Do not claim battle launch is working.

Current isolated server: `runtime/mpbt3025-combat-info`, PID 201687,
127.0.0.1:15011, world `runtime/world-test-15011-string-id.json`, traces
`runtime/traces/combat-info`, log `runtime/logs/combat-info.log`,
`-experimental-planet-room-type=1`. One client A in existing arrival-a prefix;
B stopped. Unrelated listeners 15004 and 15008 untouched. Latest all-Go tests
passed after joining/CombatInfo changes. Default server path still needs
promotion of verified staging behavior after mission validation.


## Continuation after repository move — 2026-10-23

The active server checkout is now `/home/sebulba/dev/MPBT3025-Server`, with
source in `src/` and overlay tests in `tests/`. See the
[October 23 checkpoint](solaris-session-2026-10-23.md) for the current city-roster
correction, tests, native verification and remaining work. Earlier runtime paths
and process IDs in this historical note are not current.
