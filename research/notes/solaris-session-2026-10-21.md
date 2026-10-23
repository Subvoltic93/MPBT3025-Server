# Solaris session handoff — 2026-10-21

Historical local paths in this document are relative to the original workspace root. Original client assets, client-derived research, and visual evidence remain in the separate workspace-root `client/` directory and are not included in this server project.

Current continuation: [October 23 handoff](solaris-session-2026-10-23.md) records the moved server checkout, latest tests and synchronized native armor diagnostic.

Recovery notes are chronological; read the latest checkpoints at the end first.
This workspace is not a Git checkout; no commits were made. Do not redo the
older failed numeric PlayerID/authentication experiments. All source edits are saved.

Latest continuation: native two-client Ready/NotReady and server countdown
are working; 0x80 reaches combat LOADING but a playable battle still needs
0x81 command initialization. Native room count/team/roster repairs are now
under live validation: team1/2 = Alpha/Beta, staging Alpha uses defender
count fields, native Back rejoins parent lobby, and cached peers need ARRIVED
messages during roster rebuild. Disconnect clears persisted session seats;
startup recovery preserves campaign progress and clears stale presence.
All Go packages passed. See the final checkpoints for the active test stack
and research/notes/combat-command-stream.md for recovered 0x81 command routing.

## Goal and constraints

Get two original BT3025 clients onto Solaris, visible to themselves and each
other, chatting with correct names/channels, joining the same Mission Room,
readying, and eventually starting a battle. Implement the manual's Solaris
systems using native binary/SDS evidence and live tests.

- User authorizes tests and restarting the test server and existing clients.
- At most **two game clients**. Reuse Wine prefixes arrival-a and arrival-b.
- Avoid VCR planet-state flags and populated 0x97 responses.
- Leave unrelated servers on 15004 and 15008 alone.
- CLI X11 screenshots and input work. Desktop native CUA is disabled; that does
  not prevent operating the game through the existing X11 helper.

## Verified breakthroughs

1. **PlayerID is a length-prefixed string.** Old trials wrote numeric IDs into
   string length fields, misaligning packets. `0x3c PLAYER_UPDATE` replaces
   the receiving client's own player; never send it for peers. Correct
   string PlayerIDs in `0x94`/`0x2b`/`0x2c` produced two simultaneous roster
   rows in BOTH clients, including self. Stopping B removed only B from A.
   Evidence:
   - `client/.runtime/captures/string-id-detached-a-two-players.png`
   - `client/.runtime/captures/string-id-detached-b-two-players.png`
   - `client/.runtime/captures/string-id-detached-a-peer-left.png`
   - `runtime/traces/string-player-id-detached/`
2. **0x3e UPDATE_ROOM** (reason 1, occupants-follow true) changes location.
   Full BTRoomInfo with Type 1, PlanetID 121 and city RoomID 121001/local
   moved the map into Staging Area / Missions on Solaris. Communications
   says “You are on Solaris.” EA login is not required for this local flow.
3. **0x99 READY_ROOM_ADDED** populates the staging mission list. Generic 0x27
   is accepted but does not feed that view. Correct 0x99 with full BTRoomInfo
   and empty CombatSeatLite map displayed Cadet Mission 1–3 and enabled
   Enter Mission. Screenshot `client/.runtime/captures/room-added-a-after-join.png`.
4. Clicking Enter Mission sends **0x23 JOIN_ROOM**, composite RoomID number 1,
   host local, empty password. Implemented parser, atomic balanced assignment,
   own player update and mission RoomUpdate. Actual native screen becomes
   **Mission Room**, Communications says “You are in Cadet Mission 1,” roster
   says “MechWarriors in Mission” and shows Subvoltic. Screenshot
   `client/.runtime/captures/mission-join-a-stable.png`.

## Historical failed trial — superseded by the receive-path audit below

The **Mission Room lance panels are blank and Ready is disabled**. The room
transition and mission roster work; combat configuration/seat UI does not yet.
Latest codec trial sends **0x9e CURRENT_COMBAT** with eight full CombatSeatInfo
records after 0x3e. Client accepts the traffic without disconnect/crash but
still displays no lance rows. Planet details retain placeholder labels.
Do not call this a working battle lobby yet; only one client has entered.

Latest screenshot: `client/.runtime/captures/combat-info-a-mission-live.png`.
Latest trace: `runtime/traces/combat-info/session-1792611815295721138.log`.
At 13:02:34: recv 0x23, assigned room 1 side 0 seat 0, sent own 0x3c,
0x3e, 0x9e (454-byte body), self 0x94 and self 0x2b. Client remains running.

The audit below corrected the receive-path conclusion: `0x8b` and `0x8e` are
handled by this dispatcher. Only `0x9e CURRENT_COMBAT` falls through.

## Running processes (checked at pause)

- Server PID **201687**, listening **127.0.0.1:15011**:
  `runtime/mpbt3025-combat-info -listen 127.0.0.1:15011 -world runtime/world-test-15011-string-id.json -trace-dir runtime/traces/combat-info -experimental-planet-room-type=1`
  Working dir `/home/sebulba/MPBT3025/dev/MPBT3025-Server/server`.
  Log `runtime/logs/combat-info.log`.
- ONE client: PID **201766**, callsign **Subvoltic**, window **0x420000f**,
  currently Mission Room, Wine prefix
  `/home/sebulba/MPBT3025/client/.runtime/wine32-arrival-a`.
- B prefix `client/.runtime/wine32-arrival-b` is stopped. Reuse for BetaPilot.
- Unrelated servers: PID36313 port15004; PID53869 port15008. Leave alone.
- Inspect ps/ss/window state first; PIDs may change after this handoff.

## Saved source changes this resumed session

- `src/protocol/game/pilot.go`: string PlayerID everywhere. Bootstrap
  own player/profile retain empty ID; `For(callsign)` uses callsign as local
  stable ID. Persisted numeric game.PlayerID is internal, not native wire ID.
- `src/protocol/game/pdu.go`: constants 0x3e, 0x99, 0x23; generic
  ListRoomsResponse delegates recovered RoomInfo encoder.
- NEW `src/protocol/game/room.go`: composite RoomID, full BTRoomInfo,
  RoomUpdate, generic room list, 0x99 with lite seats, strict JoinRoom parser,
  JoinRoomFailure, full CombatSeatInfo and trial CurrentCombat encoder.
- NEW `src/protocol/game/room_test.go`: fixed native RoomUpdate fixture.
- `src/protocol/game/pdu_test.go`: string PlayerID boundary regressions.
- `src/state/store.go`: factored joinRoom helper and added atomic
  JoinAvailableRoom (least populated lance, first open seat).
- `src/server/main.go`: removed peer 0x3c; opt-in room type flag;
  staging RoomUpdate; projects city mission model to native RoomInfo; 0x99
  list includes occupied lite-seat records; handles 0x23; mission hub
  presence/roster update; trial CurrentCombat broadcast to same mission.
- `src/server/main_test.go`: correct string-ID roster expectations.
- `scripts/summarize-trace.py`: parses string IDs and RoomUpdate,
  labels ReadyRoomAdded. Add JoinRoom/CurrentCombat diagnostics next.
- NEW `client/scripts/advance-solaris-client.py`: operates one specified
  existing window through recognized intro/callsign/training/map, clicks
  Solaris/GoToPlanet once, polls specified trace for 0x91, captures stages.
  Max-two-window guard. Does not launch clients.
- `client/scripts/start-solaris-test-stack.sh`: refuses room-type experimental
  server unless explicitly launched outside the default helper.
- `research/notes/bootstrap-handoff.md`: detailed recovery notes appended and
  stale numeric-ID/authentication assumptions corrected near top.

## Remaining implementation work

1. Native receive routing is verified; finish getting both peers visible in
   the lance panels after peer arrival.
2. Verify seat numbering (latest trial uses seat 0–3 plus team 0/1),
   map/climate/MechIDs. Seat state 0 renders NOT READY, 1 READY, and 2
   SELECTING. `MechID=0` leaves no selected 'Mech and disables Ready.
3. Both clients now join the same mission; verify lance roster refresh in both
   clients, chat channel cycling/delivery, and Ready PDU behavior.
4. Broadcast staging mission occupancy updates when seats change; current
   code only sends 0x99 list upon planet entry. Repeated joins/leave/disconnect
   need scoped room roster maintenance. Current hub remove/planet join still
   broad planet fanout; mission join handles same-room arrivals only.
5. ExitRoom still old generic echo and world.LeaveRoom; needs proper staging
   RoomUpdate, roster/cache restoration and hub presence update.
6. Disconnect removes hub presence but does not reliably clear persisted seats.
7. JoinAvailableRoom idempotent early return needs full eligibility validation
   if a stale seat exists; direct JoinRoom should reject AI-occupied seats.
8. Promote verified staging/mission path to defaults. Currently default
   newSessionHub.planetRoomType=-1 and CLI flag=-1: normal launch still map-only.
9. Add focused tests for new parser, balanced atomic join, occupied seat map,
   combat codec, two-player join/leave/channel behavior as implementation settles.
10. Ready/seat options, countdown/battle launch remain unwired to native client.

## Resume update — second client and receive-path investigation

At 13:08–13:10 local time, the existing arrival-b prefix was used to start
BetaPilot (PID 203831) alongside Subvoltic (PID 201766). Both are connected to
the test server at 127.0.0.1:15011 (PID 201687) and both entered Cadet Mission
1 through the native staging UI. Mission assignment is side 0/seat 0 for
Subvoltic and side 1/seat 0 for BetaPilot. The server persisted both occupants
and sent both players the peer roster. Traces:

- `runtime/traces/combat-info/session-1792611815295721138.log`
- `runtime/traces/combat-info/session-1792613272099452229.log`

Screenshots:

- `client/.runtime/captures/resume-b-mission.png`
- `client/.runtime/captures/resume-a-after-b-join.png`
- `client/.runtime/captures/resume-b-chat-tab.png`

Both Mission Room views show “MechWarriors in Mission 2” and the names Cadet
BetaPilot and Cadet Subvoltic in the room roster. The large lance panels remain
empty; each client received 0x9E with a full 8-seat CombatInfo map (454-byte
body for the first player, 472-byte body after the second joined), but the
native view did not change. Clicking Ready produced no client PDU and the
button is visibly disabled. Do not count this as ready behavior working.

Correction from the next-session audit: the earlier read of the switch table
was wrong. Virtual method 0x4DE4A0 is the lobby connection's inbound game-PDU
dispatcher; its table routes 0x8B to handler 0x4DE7D5, 0x8E to 0x4DE8B7, and
0x8F to 0x4DEA0D. Only 0x9E CURRENT_COMBAT falls to default 0x4DEF64, then
generic 0x4F3CF0, whose accepted interval is 0x06–0x48. The 0x8B handler
parses CombatInfo and inserts/updates the room-keyed combat configuration
collection; 0x8E parses a CombatSeatInfo and updates the associated room's
seat collection before notifying listeners. Thus the most recent 0x9E trial
could not populate seats. New server code now sends 0x8B with the same native
CombatInfo body plus reason 0, replacing the 0x9E trial on mission join. The
targeted codec test and full Go suite passed. Live verification of 0x8B and
the later 0x8E seat update is documented at the end of this handoff.

Chat channel cycling was opened with Tab on BetaPilot (the Communications
panel expanded and the visible selection remained “All”). Attempts to enter
text with Return and through the lower chat bar did not produce a server-side
chat PDU; neither trace contains a client 0x08 send. Channel switching and
message delivery are therefore still unverified. The input helper now accepts
an optional keysym after text, `--key=Tab`/`--key=Return`, and no-click
`--type=TEXT`/`--key=KEY` input for an already focused field. Its source is
`client/scripts/x11-game-input.c` and its local binary was rebuilt. This
supports keyboard control but did not establish chat transmission.

Historical live state before the controlled restarts; final PIDs and windows
are documented below.

## Native evidence / tools

Binary: `client/MPBT3025andGILS/MPBT3025/BT3025.exe`.
Disassembly: `client/research/raw/BT3025.exe.disassembly.txt`.
Targeted disassembly helper:
`python3 client/.runtime/dump_code.py 514ff0:80 4de4a0:50`
(arguments hex address:size).

Native writers:
- PlayerID string writer 0x4f7ff0, reader0x4f8010, vtable0x5914e4.
- RoomID 0x4f8710/0x4f8740, vtable0x591508: u32 number + host string.
- RoomInfo0x5018b0, BTRoomInfo0x51f1f0; planet is u32, not string.
- 0x3e writer0x505f90: u32 reason, bool roster follows, full BTRoomInfo.
- 0x99 writer0x527a50: BTRoomInfo + lite-seat map.
- Lite map writer0x4e65d0/decoder0x4e66d0: count + CombatSeatLite records.
- Full map writer0x42e890/decoder0x42e990: count + CombatSeatInfo records.
- CombatSeatLite0x516920: u8 seat, u8 team, string callsign, u32 house,
  u32 rank, u8 badge. Constructor0x516880.
- CombatSeatInfo0x517150 adds u8 weightClass, string occupant PlayerID,
  u32 MechID, u8 state, four u32 scores, two u16 shots, u8 kills, u32 total.
- CombatInfo0x514ff0: RoomID, u32 planet, u32 map, u32 climate, u32 terrainSeed,
  u8 field +0x38, bool sanctioned(+0x3c), bool mechVisible(+0x3d), full map.
- 0x9e CurrentCombat0x52c3b0: CombatInfo only.
- 0x8b CombatConfigurationUpdate0x52c890: CombatInfo + u32 reason.
- 0x8c SetSeatOptions0x52bc80: u8 option, u8 seat, u32 mechID, u8 seatState.
- 0x8d SetSeatOptionsFailed0x52baa0: u32 result + string reason.
- 0x8e SeatOptionsUpdate0x52b480: full CombatSeatInfo + u32 reason.
- 0x8f is a separate 9-byte structure (0x519630), not a seat list.

Build/test with bundled toolchain (system gofmt is unavailable):
`cd server; ./runtime/toolchain/go/bin/go test -count=1 ./...`
Last full suite passed after CurrentCombat trial implementation; live trial
also stayed connected. A two-player Mission Room is now verified, but no real
battle, populated lance panels, ready transition, or successful chat send is
verified.

### Screen capture/input

`xwd -silent -id 0x420000f | xwdtopnm | pnmtopng > /absolute/path.png`
Then view_image to inspect. A origin root(2,32); native800x600.
`client/.runtime/tools/x11-game-input ROOT_X ROOT_Y [ASCII_ALNUM_TEXT]`.
Enter Mission root(284,455); Back(48,455).
The startup helper raises the containing Wine desktop and focuses the window.
Do not repeatedly click a join if trace already contains it.

Launching via Python subprocess.Popen with start_new_session=True, DEVNULL
stdin and file stdout/stderr proved stable. Wine startup may take a minute.
Wait for BOTH a newly created game window AND a new server trace before using
the startup helper; selecting a stale trace caused a false timeout once.

To stop only A:
`env WINEPREFIX=/home/sebulba/MPBT3025/client/.runtime/wine32-arrival-a /usr/lib/x86_64-linux-gnu/wine/wineserver -k`
Do not wait with wineserver -w while the game is still running.

## Resume update — 13:35 local

Combat-manager receive/routing correction: native lobby dispatcher `0x4DE4A0`
routes `0x8B` to `0x4DE7D5` and `0x8E` to `0x4DE8B7`. `0x8B` stores room-keyed
CombatInfo; `0x8E` applies a CombatSeatInfo update and notifies listeners.
Only `0x9E CURRENT_COMBAT` falls through to the generic handler, which accepts
only IDs `0x06–0x48`. Sending `0x8B` instead of `0x9E` populated lance rows in
the live UI. The original two-client screenshots are
`client/.runtime/captures/config-a-two-player.png` and
`client/.runtime/captures/config-b-two-player.png`.

Native UI states were measured from live renders: state `0` displays NOT READY,
`1` READY, and `2` SELECTING. The occupied-seat encoder uses state 0. The Mech
Bay for the blank profile says “No Mech Selected”; `MechID=0` therefore leaves
the Ready button disabled on that client. A state-0 screenshot from BetaPilot
shows an enabled Ready button, but clicking did not produce a server-side
PDU. A current attempt also produced no ready PDU.

For peer arrivals, the server now sends one `0x8E SEAT_OPTIONS_UPDATE` per
occupied seat to both the joining client and existing mission peers, after
the `0x8B` room configuration. Traces confirm those 72-byte messages reach
both clients. The initial trial still rendered only each client's own seat,
so the seat key was changed from global number 0–7 to `(seat 0–3, team 0/1)`.
The latest build and live server include that change. The latest screenshot
attempt returned black frames, so the per-team numbering result is unverified.
The bottom Mission Room roster did show both pilots in valid screenshots.

Chat remains unverified: both clients display the Communications panel and the
`All: Hit TAB to cycle chat channels` hint; Tab/text/Return attempts produced
no client chat PDU in either trace. No channel change or delivery is claimed.

Source updates in this continuation:
- `src/protocol/game/pdu.go` adds handled lobby IDs for `0x8B` and
  `0x8E`.
- `src/protocol/game/room.go` factors CombatInfo encoding, adds
  configuration/seat-update PDUs, and records native seat-state constants.
- `src/server/main.go` emits `0x8B` plus occupied-seat `0x8E` updates;
  seat numbers are per team and occupied seats start at state 0.
- `src/protocol/game/room_test.go` verifies the handled `0x8B` layout.
- `scripts/summarize-trace.py` labels `0x8B`, `0x8E`, and `0x9E`.

The bundled Go suite passed after these changes with
`cd server && ./runtime/toolchain/go/bin/go test -count=1 ./...`. The built
server is `runtime/mpbt3025-combat-info`.

Current live stack:
- Test server PID **210056**, listening at `127.0.0.1:15011`, using
  `runtime/world-test-15011-string-id.json` and trace directory
  `runtime/traces/combat-info`.
- Subvoltic PID **210072**, arrival-a prefix, shell window `0x4e0000f`.
- BetaPilot PID **210074**, arrival-b prefix, shell window `0x4c0000f`.
- Each trace has a `JOIN_ROOM` (`0x23`) for Cadet Mission 1 and `0x8B`/`0x8E`
  sends. Latest traces are
  `session-1792614751076069686.log` (Subvoltic) and
  `session-1792614751030456835.log` (BetaPilot). No client chat or Ready PDU
  has been observed.
- Unrelated servers remain PID36313 on port 15004 and PID53869 on port 15008.

Keep the two-client limit. Inspect before restarting; leave both unrelated
servers alone. Do not use VCR flags or populated `0x97` responses.

## Resume update — native chat wire format, 13:40 local

A live BetaPilot input test found the chat field: click the bottom message line
(root coordinate about `200,599` for the 800×600 shell), type text, and press
Return. The client emitted `0x08 CHAT_MESSAGE` with channel byte `0` (All).
The capture exposed the native body as message type byte 1, sender PlayerID
string, a 4-byte zero field, sender callsign string, 20-byte sender record,
length-prefixed text, and channel byte. The previous parser expected a sender
at body offset 7 and rejected this valid message. `src/protocol/game/chat.go`
now parses and canonicalizes the captured layout; tests include the live
BetaPilot “Hello” packet. Full Go tests passed and the server binary was rebuilt.

The corrected chat parser has **not** been live-verified for relay or Tab
channel changes yet. In particular, the only observed client send was from the
earlier process before the parser fix, and the server logged it as malformed.

After the parser rebuild, both existing arrival-prefix clients were relaunched.
Current processes at pause: server PID **211311** on port 15011; arrival-a
Subvoltic PID **211328** (window `0x4c0000f`); arrival-b BetaPilot PID **211330**
(window `0x500000f`). The test server has only two clients. Subvoltic has
completed character creation, but the latest traces do not yet show
`JOIN_PLANET` or `JOIN_ROOM` for this final process pair. Startup screens are
currently returning black X11 captures, so this pair is not yet back in the
Mission Room. The immediately preceding pair had both players in Cadet Mission
1; their verified roster and seat-state screenshots are listed above.

## Resume update — post-reboot checkpoint, 14:xx local

The workstation required a hard reboot during the previous continuation. On
resume, process inspection found no MPBT3025 server/client processes, and
`ss` found no listeners on 15011, 15004, or 15008. Treat all prior PIDs and
window IDs as stale. No game client or server has been relaunched in this
continuation. The unrelated services were not touched; their previous PIDs
cannot survive the reboot and they may be managed elsewhere.

Recovery is proceeding from the saved source and traces first. Before any
future client launch, the static receive/routing path will be reviewed and the
server-side stack will be checkpointed with bounded logs and process counts.
Avoid repeated captures or high-frequency polling; use one capture per
milestone if live verification is needed.

## Resume update — receive-table correction, 13:48 local

The prior claim that dispatcher `0x4DE4A0` routes wire IDs `0x8B` and `0x8E`
to `0x4DE7D5` and `0x4DE8B7` is incorrect. Its switch computes `ID - 0x86`;
the byte table sends both `0x8B` and `0x8E` to default `0x4DEF64`. The named
targets correspond to other entries (`0x4DE7D5` is table ID `0x95`,
`0x4DE8B7` is `0x98`, and `0x4DEA0D` is `0x99`). The `0x99` ready-room
handler mapping is confirmed. Do not rely on the old dispatcher interpretation.

A separate dispatcher at `0x40F3A0` uses another `ID - 0x86` table. Its
`0x8B` entry reaches `0x40F4DE`; `0x8E` reaches its default branch. That
dispatcher has special filtering for `0x8B`, so the exact callback/manager
that consumes the decoded CombatInfo still needs to be traced. The fact that
prior captures showed lance rows after a `0x8B` send is live evidence, but it
does not identify which receive path produced the view. No runtime was started
while auditing these tables.

Re-inspection of `config-a-two-player.png` and `config-b-two-player.png`
confirms that both clients displayed BetaPilot in the upper lance and Subvoltic
in the lower lance, with both names also present in the two-player mission
roster. The body sent by `0x8B` contained both CombatSeatInfo records. These
screenshots establish that the configuration update was sufficient to render
both seats, despite the misleading first-dispatcher table result; the exact
internal route remains unresolved. They also show each occupied seat as
`READY` while the server encoded seat state `0`. The old `0=NOT READY` claim
was inferred incorrectly and is withdrawn. The bottom Ready button appeared
disabled, so user-ready transition remains unverified.

Post-reboot validation checkpoint: `go test -count=1 ./...` passed with the
bundled Go toolchain. The saved test world still has Subvoltic and BetaPilot in
Cadet Mission 1, each in seat 0 on opposite teams. Joining Solaris again runs
the store's room-removal path before a new native join, so that persisted test
state can be reused. Next runtime milestones are: start only the 15011 test
server; launch arrival-a and advance with one screenshot per screen; then do
the same with arrival-b. Never run the older continuous-capture advance helper
for this recovery test.

Runtime milestone 1 at 13:52 local: rebuilt `runtime/mpbt3025-combat-info`
and started only that binary on `127.0.0.1:15011` (PID 4207). Arrival-a is the
only game client (PID 4262, window `0x420000f`), using
`client/.runtime/wine32-arrival-a`. The window initially captured black at
startup; one scripted intro click revealed the call-sign/map screen, after
which Subvoltic creation, training exit, Solaris selection, and one Go To
Planet click succeeded. Trace
`runtime/traces/combat-info/session-1792615814681913128.log` has exactly
one `0x91 JOIN_PLANET` and the Solaris `0x3e`, occupant `0x94`/`0x2b`, and four
`0x99 READY_ROOM_ADDED` sends. Recovery captures are under
`client/.runtime/captures/recovery-a-*.png`. No second client has been launched
in this post-reboot run yet.

Runtime milestone 2 at 13:54 local: started arrival-b (PID 4591, window
`0x4c0000f`) from `client/.runtime/wine32-arrival-b`. Its initial splash and
intro advanced normally after a single click. BetaPilot completed character
creation and training exit, then one Go To Planet click produced exactly one
`0x91 JOIN_PLANET` in trace
`runtime/traces/combat-info/session-1792615940595720206.log`. The
server sent both occupant records to B and published four `0x99` mission
entries. Both game clients remain under the two-client cap; next milestone is
joining Cadet Mission 1 from B and checking whether A gets the two-seat
`0x8B` update.

## Resume update — unique native seat keys, 14:02 local

After recovery, the live test server on 127.0.0.1:15011 and exactly two clients
(Subvoltic/arrival-a and BetaPilot/arrival-b) were brought back into Cadet
Mission 1. Both server traces recorded the room join and two-seat CombatInfo
updates. The per-window screenshots showed each client listing both pilots in
the mission roster but only its own pilot in a lance seat. This narrowed the
problem to how the client stores/decodes seat records, rather than room joining
or the visible mission roster.

Native analysis found the combat seat map comparator reads only
`CombatSeatLite.Number` at offset +4; it does not include Team. Our team-local
numbering gave both occupied records Number=0, so one key overwrote the other.
Changed `missionCombat`, `missionSeatUpdates`, and `missionSeatLites` to encode
unique numbers 0–7 (`side*4 + lance index`) while preserving Team 0/1. Added a
regression test that checks the two occupied wire keys are `(0,0)` and `(4,1)`
and that all eight CombatInfo keys are unique. `go test -count=1 ./...` passed.

The next step is to build the updated server, stop only the current 15011 test
server and the two arrival-prefix clients, then relaunch this same pair and
verify the two lance rows. Keep unrelated servers untouched. If both seats
render, continue with one-shot checks for chat channel switching and Ready.

## Resume update — asymmetric seat rendering after unique IDs, 14:04 local

The rebuilt server and both arrival-prefix clients were restarted cleanly. The
clients returned to their startup flow, so Subvoltic and BetaPilot were
recreated/reused and both joined Solaris, then Cadet Mission 1. Server traces
confirm one `0x23 JOIN_ROOM` per client, `0x8B` CombatInfo bodies containing
both occupied records at unique keys `(0,0)` and `(4,1)`, and matching `0x8E`
seat updates. The native map-key collision is fixed and covered by the passing
Go test.

Live UI verification is asymmetric: BetaPilot shows Subvoltic in the upper
lance and BetaPilot in the lower lance; Subvoltic shows only its own upper
lance seat while the lower lance remains empty. Both Mission Room bottom
rosters show two pilots. Therefore unique global seat numbers alone did not
fully solve peer seat rendering. The A trace confirms it was sent the complete
updated 0x8B and both seat updates, so continue tracing the receive/update
handling or client-side view refresh. Both clients remain alive; server is
PID 5792. Current game PIDs are 5999 (arrival-a) and 6153 (arrival-b), with
windows 0x420000f and 0x4e0000f. Avoid losing this evidence; captures are
`recovery-global-a-two-seats-later.png` and `recovery-global-b-two-seats.png`.

## Resume update — live chat packet corrected, 14:07 local

BetaPilot's live room All-chat packet was captured exactly as a 64-byte body.
The second length-prefixed callsign follows immediately after the first
PlayerID string; there is no intervening four-byte gap. The current chat codec
incorrectly expected that gap and logged `chat callsign length 606821`; neither
client displayed `AllCheck`. Corrected the offsets and encoder, and updated the
protocol regression fixture to match the live shape. The complete Go suite
passes again. This correction is in source but has not yet been deployed to the
running test server; rebuild/restart the isolated 15011 stack, then verify All
relay and channel filtering.

Ready-state observation: both clients display `NOT READY`; BetaPilot has an
enabled Ready button while Subvoltic's button is disabled. A single click on
BetaPilot's button left the UI unchanged and produced no new server-side game
PDU. The `0x8C` seen in the BetaPilot trace was sent immediately upon entering
the room, before this click. Exact Ready control behavior remains unverified.

## Resume update — exact two-byte chat separator, 14:10 local

Correction to the 14:07 note: the captured BetaPilot body does contain a
separator, but it is two zero bytes (u16), not the four-byte gap first tried
and not zero gap. The exact body bytes at offsets 14–17 are `00 00 00 09`:
two separator bytes followed by the second string's four-byte length. The
one-byte-too-long parser read `00094265` (606821); the zero-gap retry read a
zero length. The source now uses a two-byte gap, and `ChatMessageFor` encodes
it. The live-capture fixture and full Go suite pass; the server binary was
rebuilt. The current live server still runs the prior binary, so the next step
is to restart the isolated two-client stack and verify relay.

## Resume update — live chat record length corrected, 14:12 local

Correction to the prior chat checkpoint: the second live All message confirmed
the two-byte separator, but text parsing still failed because the native sender
record occupies 23 bytes after the second callsign, not 20. The live body has
the four-byte text length at offset 52 for a 9-byte callsign; this is
`callSignEnd + 23`. Updated `chatSenderInfoLength` and the protocol encoder.
`go test -count=1 ./...` passed, and the server binary was rebuilt. The running
server is still the pre-fix binary; relaunch the two-client stack and retest.

## Resume update — exact chat sender record is 22 bytes, 14:15 local

Correction to 14:12: the second chat capture showed the text-length field at
body offset 51. With the second callsign ending at offset 29, the fixed sender
record is 22 bytes. The previous 23-byte assumption read the first text byte as
part of the length (for example `00000a41`). Updated the serializer/parser
constant to 22 bytes and the wire fixture. The full Go suite passes and the
server binary was rebuilt. One final live All/Team relay pass remains.

## Resume update — All chat parses and routes to both sessions, 14:18 local

With the rebuilt 22-byte chat layout, BetaPilot sent `All22`. The server parsed
the live packet without an error and wrote the canonicalized message to both
connected session traces (`0x08`, channel 0). BetaPilot's UI displays the
message; Subvoltic's UI capture did not visibly show it, despite a server send
in that session trace, so peer rendering still needs investigation. The client
shows the sender prefix twice (`BetaPilot: BetaPilot: All22`), indicating the
server's added text label duplicates the native sender label on the sender
view. Current two clients are both in mission 1 on opposite sides; BetaPilot
side 0 seat 0, Subvoltic side 1 seat 0. The server has the unique-key 0x8B
map and 22-byte chat parser build.

## Resume update — chat routing by channel, 14:19 local

The final live packet layout now parses successfully. BetaPilot's All channel
byte 0 produced two server sends, one to each session trace, while House
channel byte 2 produced one send (sender only; neither cadet has a House).
The Mission Room UI cycles between All and House with Tab. BetaPilot's chat
pane shows both messages, but each has the sender label duplicated by our
server text prefix; Subvoltic's pane remains blank despite the All PDU in its
session trace. Thus server-side channel routing works, but peer-side chat
rendering and the duplicate label still need a client-compatible relay shape.
Both pilots remain in the same room on opposing sides and both seat records
appear in the 0x8B wire map; each client view renders only its own pilot seat.
Current readiness remains NOT READY: Subvoltic shows an enabled Ready button;
BetaPilot shows it disabled. Next inspect a single click on Subvoltic's enabled
button and the resulting PDU/state.

## Resume update — Ready button wire event, 14:20 local

Clicking Subvoltic's enabled Ready button emitted client PDU `0x8C` with body
`02 00000000 0001` (13-byte wire PDU including its header). The server logs
`0x8C` but has no handler; the button remains enabled and the seat remains
NOT READY. This distinguishes the real Ready action from the automatic `0x8C`
sent on room entry (`01 00000000 000800`). Trace the native `0x8C` writer and
receiver, then add the smallest server transition and confirm peer/UI state.

## Resume update — server Ready handling and chat label fix, 14:23 local

Native 0x8C audit: the Mission Room Ready click is subtype 2 with a zero u32
and trailing u16 1. A distinct subtype-1 (`01 00000000 0800`) is sent
automatically on room entry, so it must not toggle readiness. Client dispatcher
0x40F3A0 maps only its selected IDs; 0x8C goes to its default path. Server now
parses only the captured 7-byte subtype-2 request, calls `Store.SetReady`,
reflects stored readiness in 0x8B/0x8E seat state, and broadcasts the updated
seat to sessions in the same mission. Type 1 and malformed requests are
ignored. Added parser tests for ready, unready, type-1, invalid value, and bad
length.

Live chat showed native client already renders the canonical callsign from the
sender fields. Removed the extra text prefix that produced
`BetaPilot: BetaPilot: All22`; sender text is preserved and only identity
fields are canonicalized. Full Go suite passed and server binary rebuilt. The
current live test server still uses the prior binary; restart the 15011 stack,
rejoin both existing prefixes, then verify Ready status on both screens and
whether the corrected All message is visible to Subvoltic.

## Resume checkpoint — post-hard-reboot recovery, 14:xx local

After the host hard reboot reported by the user, the 15011 test server and the
same two arrival-prefix clients were relaunched. Current listener is only
127.0.0.1:15011 (server PID 11384); the two game processes are PIDs 11385 and
11386. No additional game clients are running. The rebuilt binary with Ready
handling and canonical chat text is active. Before further UI interaction,
verify both client windows and capture their current state; continue with one
interaction at a time and record each result here.

## Resume checkpoint — both pilots in Solaris staging, 14:xx local

Recovered UI state and advanced the existing two clients (arrival-a PID 11385,
arrival-b PID 11386) through startup without restarting the machine. Both
clients are at Solaris staging with Cadet Mission 1 selected. A's Mission
Room roster lists Cadet BetaPilot; B's roster lists Cadet BetaPilot and Cadet
Subvoltic, and the planet roster count is 2. The server received both callsign
requests (`0x87`, BetaPilot/Subvoltic) and sent `0x88` responses. Next enter the
mission from each client and validate the room-seat and ready/chat behavior.

## Resume checkpoint — live roster, chat filters, Ready route, 14:27 local

Both existing clients are in Cadet Mission 1 and the planet roster count is 2.
Each screen shows the same two pilots in the lower mission roster. The combat
seat display remains asymmetric: Subvoltic appears in the upper lance on one
client and BetaPilot in the lower lance on the other. BetaPilot's Ready click
sent `0x8C` subtype 2; the server stored Ready and sent an `0x8E` update with
seat state byte 1 to both sessions, but both client screens still say NOT
READY. `0x8E` is default/unmapped in the audited client receive table, whereas
`0x8B` maps to combat configuration. Added a Ready-path refresh to publish a
fresh full `0x8B` combat snapshot to both mission sessions alongside the
existing `0x8E` notice; rebuild/restart and retest this update next.

Chat verification on this run: All (`0`) showed `AllCheckpoint` on both
clients; the early peer capture was simply too soon. Tab cycles All (`0`) ->
Team (`1`) -> House (`2`). Team (`TeamCheckpoint`) and House
(`HouseCheckpoint`) showed only on BetaPilot because the pilots are on
opposing sides and different Houses. The sender prefix appears once, and the
server traces confirm channel-specific delivery. The latest screenshots were
captured in the user-visible UI; use them to compare after the 0x8B Ready
refresh.

## Resume checkpoint — binary rebuilt, clients exited on disconnect dialog, 14:29

The 0x8B Ready refresh is in the rebuilt binary and all Go packages pass. I
restarted only the test listener on 127.0.0.1:15011 (now PID 12543); the saved
world loaded 121 planets, 457 rooms, and the two pilots. Existing game clients
showed a disconnect dialog and exited when it was dismissed. There are
currently zero game clients and no other server was stopped. Relaunch only the
arrival-a and arrival-b prefixes, confirm new traces/windows, then repeat the
Mission Room Ready transition against the refreshed build.

## Resume checkpoint — Ready refresh build and safe stop, 14:33 local

Added `sessionHub.publishMissionCombatUpdate` and the Ready path now sends a
fresh `0x8B` combat configuration to every session in the mission after the
server updates stored seat readiness, in addition to the `0x8E` seat notice.
This follows the native receive path where `0x8B` reaches combat
configuration, while `0x8E` fell through the audited secondary dispatcher.
Extended `TestSolarisMissionSeatNumbersAreUniqueAcrossLances` to assert that
mission seat updates expose BetaPilot as READY and AlphaPilot as NOT READY
after `Store.SetReady`. `gofmt`, `go test -count=1 ./...`, and a fresh server
build all passed.

I restarted only test server 15011 as PID 12543; it loaded the two saved
pilots. The prior pair disconnected when that server restarted. I relaunched
only arrival-a and arrival-b (PIDs 12659 and 12914, never more than two), but
both client windows stayed black after fresh network handshakes; arrival-a
reached callsign request `0x87` for Subvoltic, and arrival-b connected and
exchanged initial Aries setup. After a bounded wait, both game processes were
using roughly one CPU core apiece. Stopped them with their prefix-specific
Wine `wineserver -k`; there are now zero clients and the 15011 test server
remains listening. No unrelated servers were stopped. This leaves the `0x8B`
Ready refresh built and unit-tested but not live-verified; restart the same two
prefixes from a stable render state and check whether NOT READY changes. Do
not exceed two clients, and continue avoiding VCR flags and populated `0x97`.

Prior live evidence remains valid for the previous build: both pilots reached
the same Cadet Mission 1; the lower mission roster listed both callsigns;
All chat rendered on both screens, Team and House chat were sender-only for
these opposite-side, different-House pilots, and the sender label appeared
once. The Ready click sent the captured subtype-2 `0x8C`, which the previous
server translated into a stored READY state and `0x8E` sends. The UI stayed
NOT READY after `0x8E`, which motivated the new `0x8B` refresh. Per-client
lance-seat views remained asymmetric despite both seats in server snapshots.

Preserved the successful-run screenshots in
`client/.runtime/captures/solaris-session-2026-10-21/`: both Mission Room
views, Ready after the click and its peer view, and sender/peer views for All,
Team, and House. In the successful run, the BetaPilot view displayed both
upper Subvoltic and lower BetaPilot seats, while the Subvoltic view displayed
only Subvoltic in the upper seat despite the two-pilot lower roster. Use these
images as the before snapshot for the next receive-path investigation.

## Recovery checkpoint — following another host crash

No game/server processes or test listeners survived. Saved Ready refresh source
passes all Go tests and was rebuilt. Restoring only test listener 15011 and
arrival-a first, checking rendering before starting arrival-b. Previous chat
verification remains valid; Ready refresh and peer seat refresh need live checks.

Recovery milestone: arrival-a rendered normally and joined Cadet Mission 1.
Capture recovery-ready-a-mission.png displays Subvoltic NOT READY and saved
BetaPilot READY, confirming full snapshots can render READY. BetaPilot is
not connected yet; these are persisted seats. Launching arrival-b next.
Server PID 3539, client A PID 3540, trace recovery-ready/session-1792618828084176301.log.

## Recovery milestone — live Ready refresh, 14:42 local

Both recovered clients joined Cadet Mission 1 (Subvoltic side 0 seat 0,
BetaPilot side 1 seat 0). BetaPilot Ready click at 14:42:24 emitted
0x8C body 02000000000001; server stored READY and sent 0x8E plus full
0x8B to BOTH sessions. Subvoltic peer capture recovery-ready-a-peer-ready.png
shows both lance seats and BetaPilot READY. This verifies peer refresh after
0x8B; the immediate sender capture was too early and needs a settled check.
Server PID 3539; clients A 3540/window 0x420000f, B 3835/window 0x4c0000f;
Wine desktops 0x3c00007 and 0x4600007. Traces under recovery-ready.

## Native combat-manager discovery — seat zero sentinel

Actual combat manager constructor 0x4d6af0 installs vtable 0x590d30.
Receive method 0x4d8590 dispatches ID-0x80 through byte table 0x4d8a34
and jump table 0x4d8a00: 0x8B -> 0x4d8710, 0x8E -> 0x4d883b.
Thus 0x8E IS handled here; prior secondary UI/default observations were
incomplete. Both update CombatInfo at manager +0xe4 and emit events.
Local-seat updater 0x4d8a70 stores seat number at +0x1b4 and tests zero
as unseated (0x4d8a8a / 0x4d8a92). Our first seat 0 therefore cannot
represent a seated pilot. Changed native seat numbering to 1..8 (model
indexes remain 0..3); adjusted the unique seat regression. Live test pending.
BetaPilot own view still NOT READY after settled foreground checks while
Subvoltic peer view displays BetaPilot READY; this is not full Ready success.

One-based live result: Subvoltic first seat now enables Ready and sends
the automatic subtype-1 0x8C on room entry (absent with seat zero).
Capture recovery-seat-one-a-mission.png. Server PID 4639, A PID 4640;
trace recovery-seat-one/session-1792619179577493530.log.

## Recovery checkpoint — one-based Ready verified, 14:49 local

Both pilots are in Cadet Mission 1 using wire seats 1 and 5. Subvoltic Ready
at 14:47:00 and BetaPilot Ready at 14:49:11 each emitted subtype-2 0x8C.
Server stored READY for both and broadcast 0x8E plus 0x8B. Subvoltic capture
recovery-seat-one-a-both-ready.png shows BOTH lance rows READY, both pilots
in the lower roster, and a Not Ready button. This is stronger live evidence
than the previous run. BetaPilot immediate own row still says NOT READY;
local button/row redraw must be checked after the foreground settles.
Earlier Subvoltic own row also lagged until the peer action, even though
its button had already changed to Not Ready. Do not claim immediate
symmetric redraw or battle launch.

Additional native correction: 0x9E maps to combat-manager 0x4d8696 and
replaces snapshot +0x148; 0x8B maps to 0x4d8710 and replaces +0xe4.
The protocol comment now reflects this rather than claiming 0x9E is dropped.
This may matter when tracing stale local-row refresh. 0x8E maps to
0x4d883b. Event constants: manager config=0x8a, seat update=0x8b, local
ready=0x8c, local seat=0x8e. Do not confuse these EVENT IDs with WIRE IDs.

Active stable stack: server PID 4639 on 127.0.0.1:15011, A PID 4640/window
0x420000f (desktop 0x3c00007), B PID 4911/window 0x4c0000f (desktop
0x4600007). Same two arrival prefixes, no others. Logs recovery-seat-one.log
and recovery-seat-one-{a,b}.log; traces recovery-seat-one/session-
1792619179577493530.log (A), 1792619245624253269.log (B). Inspect before
restarting. Go suite passed after the one-based seat change; binary rebuilt.
No VCR flags or populated 0x97 used.

Final settled check: recovery-seat-one-b-settled-final.png still shows
BetaPilot own NOT READY and Ready button, whereas A shows BOTH READY
and Not Ready. Server state has both ready. So sender state redraw is
confirmed unresolved, not just an immediate-capture artifact. Leave the
current stable pair for inspection; no additional clients launched.
Native UI modal handling at 0x457fc2 checks manager +0x1b4 !=0 before
calling 0x4d8db0 with state 4; 0x45803e similarly sends state 0 on close.
This independently confirms the seat-zero sentinel. Trace local state
event 0x8c consumers and +0xe4/+0x148 snapshots next.

## Continuation checkpoint — focus guard and native options layout

Added client/scripts/operate-solaris-client.py: raises the actual X11 root
frame ancestor, verifies XGetInputFocus, and combines local-coordinate input
with capture. The old runtime focus-capture.py only raised the nested Wine
desktop; one attempted chat probe hit Codex. Correct chat input is local
(200,567), root (202,599), NOT local y591 ticker. Verified focused chat
ReadyRefreshProbe and a Ready click reached the game, sender stale state remains.

WineDbg attach caused a WoW64 debug-break-thread page fault; do not repeat
attach. Stopped only B prefix to clear it and relaunched B (PID 6759).
A PID4640 and server4639 untouched. Built a scratch read-only Windows PE
helper with local GNU as/ld and ReadProcessMemory; it reads game memory
without debugger attachment, tested on A successfully. Script:
client/.runtime/read-wine-memory.py (prefix, pid Windows0x20, address,size,out).

Important native 0x8C writer correction: 0x52bc80 writes BYTE subtype at+8,
BYTE seat at+0xc, MechID object u32 at+0x10 (value+0x14), BYTE state+0x18.
So body is subtype/seat/u32MechID/state, NOT subtype/u32zero/u16state.
Automatic 01000000000800 means subtype1, seat0, MechID8, state0. The server
currently ignores this selection and sends MechID0. Ready type2 parser only
works accidentally for captured zero-seat/zero-mech requests. Correct parser
and selection persistence need implementation. A native manager memory
shows own MechID8, seat1, state1 (manager0x1743e30 +1b0/+1b4/+1b8).

## Implementation checkpoint — native Mech selection preserved

Corrected 0x8C parser to subtype/seat/u32MechID/state from writer0x52bc80.
Added typed SeatOptionsRequest and fixtures including automatic ID8 selection
and nontrivial uint32 bytes. Added Seat.MechID native ID persistence and
Store.SelectMissionMech, enforcing session pilot occupancy. Mission full
snapshots and seat updates now preserve selected native MechID. Subtype1
selects Mech and echoes its occupied seat; type2 still accepts Ready/NotReady
only. Transient states and seat moves remain unimplemented. Added persistence
and foreign-seat rejection test. Binary rebuilt; controlled restart next.

## Critical codec correction — combat planet was zero

Read-only client memory inspection confirms the native manager already has
state1 and MechID8 after Ready, but subject notification guard +0x10 stays1
(after entry; before entry0). Map records contain state1 too. This indicates
suppressed subsequent notifications, not a missing state write.

Native CombatInfo layout was misidentified. Writer0x514ff0 sends RoomID+4,
numeric wrapper+1c, PlanetID+24, MapID+2c, u32 terrainSeed+34, BYTE climate
+38, bools+3c/+3d, seat map+40. Formatter0x515144 reads value+28 as
m_planet, 0x515185 reads+30 as m_map, and0x51527d reads+38 as m_climate.
Our prior layout wrote 121 into the +1c wrapper and ZERO into real PlanetID,
causing Mission Room callbacks to look up planet0 despite a valid Solaris121
cache. Placeholder planet/map labels and stuck observer guard follow.
Corrected codec to zero default +1c, planet121, map0, seed0, byteclimate0.
Added explicit nonzero-map/climate native byte fixture; build/live check next.

CombatInfo correction: all Go packages passed, binary rebuilt. Restarted
only the15011 stack with the same two prefixes. Logs/traces combat-planet.

## Verified breakthrough — native Ready redraw fixed, 15:11 local

Corrected CombatInfo now resolves real PlanetID121. Native mission shows
Cadet Mission1, Planet Solaris, and selected COM-4D (25 tons). After settling
terrain loading, manager notification guard+0x10 is ZERO (was stuck1 with
PlanetID0). A Ready click immediately shows Subvoltic READY and Not Ready
button: client/.runtime/captures/combat-planet-a-ready.png. Thus the stale
sender UI was downstream of the misencoded planet/config callback, not a
need for artificial delays. No transport delay or duplicate-refresh hack added.

Server PID8643, A8644/window0x4a0000f, B8646/window0x4c0000f. Traces
combat-planet/session-1792620626678883395.log(A),1792620626982997407.log(B).
A is in mission1, B startup helper advancing; wait until helper finishes
before other UI interaction. Only these two clients.

## Ready roundtrip confirmed — 15:13 local

Both native clients joined mission1 with corrected CombatInfo. B Ready at
15:12:57 and NotReady at15:13:15 sent subtype2 state bytes1/0 and update
both screens, including own Not Ready/Ready button. A then un-readied too,
leaving both NOT READY and clearing model countdown. Captures combat-planet-
{a,b}-ready.png, b-not-ready.png, a-peer-not-ready.png, a-not-ready.png.
A shows COM-4D/25tons for both; B sometimes still shows Any Mech, despite
persisted native_mech_id8 for both. That cache/display difference is separate
from now-working Ready controls.

Native countdown recovered: COUNTDOWN_TIMER_PDU ID0xA9, writer0x523250
=u32seconds then BYTE kind. Manager4d8590 emits event0x95; room UI460a55
reads fields+8/+c, starts a1sec timer. UI4630b0 labels kind3/default
Auto Launch in, kind4 GET READY, kind1/2 opposing victory timers; kind0
clears countdown text. kind3 near3seconds triggers UI launch action.
Next wire countdown to Store.CountdownEnds; do not fake combat launch.

## Countdown implementation checkpoint — before restart

Added native0xA9 CountdownTimer(u32seconds,bytekind), constants stopped0
and AutoLaunch3, native byte fixture. Ready updates now broadcast remaining
Store.CountdownEnds seconds to mission sessions; ceil rounding preserves
partial seconds. Second-ready and unready-with-other-ready preserve original
deadline. All-unready emits zero/stopped. Joining clients receive current
timer too. Added a store-to-wire shared-deadline/cancel test. Trace labels
now recognize0x83 START_COMBAT and0xA9 COUNTDOWN_TIMER. Native StartCombat
request descriptor0x5945e8 confirms wireID0x83 (no body); still unhandled.
Do not claim battle launch until native engine connection is implemented.

## Native auto-launch request and live launch attempt — 2026-10-22

The prior countdown-native trace received 0x8C body 02000000000003 about
68 seconds after Ready. This matches the native 60-second timer expiring:
the client emits subtype2/state3 as its auto-launch request. Updated the
server to recognize this request, run StartExpiredBattles, and send the
native 0x80 ENTER_COMBAT message (battle type0, teams-rebalanced false) to
the mission's connected clients. This server build compiled successfully.

Recovered after another host restart, reusing only arrival-a/Subvoltic and
arrival-b/BetaPilot on 127.0.0.1:15011. Current server PID209042; A PID209104
window0x4a0000f; B PID209105 window0x4c0000f. Both are in Cadet Mission1,
Solaris, wire seats1 and5, MechID8, COM-4D. Both Ready actions were received
at 10:55:45 and10:55:49 local. Each screen shows both rows READY; A's native
countdown visibly reads “Auto Launch in: 51 sec.” Trace directory
runtime/traces/countdown-launch; A session-
1792691624584789028.log, B session-1792691627756123849.log. This is a live
attempt only: wait for native state3, check the server's room-start log,
0x80 delivery, and whether both clients enter the combat UI before calling
the battle launch verified. The new server build has not had its existing
Go suite rerun after the auto-launch edit.

The timer reached 00:00 on A but neither trace contained an outgoing state3;
the room remained in countdown. Both pilots then clicked Not Ready; the server
received state0 from each, emitted stopped countdown PDUs, and persisted room1
as open with both seats not ready. This live cancellation is confirmed. Added
a one-second server scheduler to call StartExpiredBattles and broadcast
0x80 ENTER_COMBAT for every newly expired room. Rebuilt successfully. The
currently running process is still the pre-scheduler build; restart the test
stack (only port15011 and the same two prefixes) to validate server-owned
expiry. A/B UI may now show disconnection after restart.

Restarted from the clean open room with server PID213758 and the same two
clients: arrival-a/Subvoltic PID213973 window0x4e0000f; arrival-b/BetaPilot
PID213974 window0x4a0000f. Both rejoined Mission1 and selected MechID8. Both
Ready clicks have been accepted on the scheduler build. Countdown expiry
validation is now in progress; log is
runtime/logs/countdown-scheduler.log, traces are in
runtime/traces/countdown-scheduler.

Live expiry succeeded at 11:02:29: the server log records “Mission 1
countdown expired; entering combat,” then sends an 8-byte Aries frame to
each client containing wire PDU0x80 and body 0000. Both client TCP sessions
remain established. Read-only process-memory inspection shows each native
combat manager has entered-combat battle type0, own MechID8, own seat1/5,
and ready state1. Immediately after receipt both old Mission Room X windows
became unmapped; no new visible game window appeared, and the game processes
remain live. This confirms packet delivery and native manager handling, but
the playable combat UI is not yet verified. Next inspect the native 0x80
transition's required combat snapshot/engine initialization and the unmapped
window state before claiming combat reached.

Correction after capturing each mapped Wine desktop: both clients did create
the native 3D “LOADING…” screen immediately after their old Mission Room
windows were unmapped. The 800x600 Wine desktop captures are
client/.runtime/captures/countdown-scheduler-{a,b}-desktop-after-expiry.png.
So 0x80 successfully advances both clients into combat loading; completion
and controllable gameplay are the remaining checks.

Combat DLL routing: BT3025.exe's 0x81 receive branch calls the imported
BTCombat::CRealityManager::ReceiveCombatData(data,length), passing the opaque
PDU body directly. The DLL parses that command stream into combat objects.
The scheduler currently sends 0x80 only; the room stays on its 3D LOADING
splash after several minutes, so initial 0x81 combat data is likely the next
missing input. 0x8B mission configuration and 0x80 are both confirmed received;
do not treat the splash as a loaded or playable battle. Continue reversing the
0x81 data command format and the minimum title/terrain/Mech commands needed to
complete initialization.

Experimental follow-up: the scheduler build was rebuilt to send a native
0x9E CurrentCombat snapshot (same room/planet/map/climate/seat data) to each
room client immediately before 0x80. The current live pair remains on the
earlier build in the loading splash. For the next isolated run, clone the
persisted world to a second test-world file and reset only room1 to open with
all pilots not ready; preserve the original in-combat snapshot. Reuse the two
same prefixes, then compare whether 0x9E before 0x80 advances loading. Server
build completed; no Go test suite rerun after the new scheduler/snapshot edits.

Follow-up result at 11:13:18: on the isolated world clone, both clients again
joined Cadet Mission1 as Solaris pilots Subvoltic and BetaPilot, selected
native MechID8 (COM-4D), and became ready. The server expired the countdown,
sent each client the 472-byte 0x9E CurrentCombat PDU, then the 0x80 EnterCombat
PDU. Both mapped Wine desktops still showed the identical “LOADING...” splash
after the transition; `0x9E` alone does not complete combat initialization.
Screenshots are `client/.runtime/captures/current-combat-snapshot-{a,b}-after-expiry.png`.
The live stack is server PID219165 on 127.0.0.1:15011 with client PIDs219220
and219221; its trial world clone is
`runtime/world-test-15011-current-combat-snapshot.json`. Only the
two assigned clients are connected.

Static parser inspection: `CRealityManager::ReceiveCombatData` at image VA
0x6905cd60 forwards the body pointer and byte count to 0x690755c0. That parser
constructs a bounded input stream, reads a command-type byte as an encoded
character (`wire byte - 0x21`), looks up a command factory in a registry, then
parses and dispatches each recognized command. The serializer helper adds
0x21 back to command-type bytes. The dispatch callback is called with two
context values, and exported `FillReality(TitleCommand&)` and
`FillMech(NewMechCommand&)` identify at least the title/configuration and
Mech-creation commands needed by the engine. The type registry and command
payload encoders still need to be recovered before constructing a safe 0x81
stream; no speculative 0x81 packet has been sent.

## Room presence/count/team repair — 2026-10-22, 12:00 local

User reported room counts, unexpected players, and incorrect team headings.
Native screenshot current-combat-snapshot-a-ready.png confirms top heading
blank and lower heading Lance Alpha. UI code at 0x460da9 checks team1 for
Alpha, team2 for Beta; zero has no heading. Changed all mission seat codecs
(full CombatInfo, seat updates, staging lite records) to teams1/2 while
retaining wire seat numbers1..8.

Found no live mission-list updates after joining/leaving, no persisted-seat
cleanup on TCP disconnect, and no startup cleanup after a host crash. Added
session cleanup on disconnect and explicit ResetSessionPresence at server
startup (campaign progression and planet/city remain). Added native
READY_ROOM_UPDATE0x98 (descriptor0x5940c0, writer0x5280b0, same body as0x99)
and READY_ROOM_REMOVED0x9a (writer0x5276a0, RoomID only). Staging viewers now
receive updates as membership changes; unknown/new empty missions use0x99.
Mission departure refreshes remaining clients' full seat maps/countdowns,
and unrelated room/planet arrivals no longer leak into a mission roster.
Team regression expectations corrected. Added lifecycle regressions for live
room updates/disconnect release, unrelated roster events, and persisted
crash recovery. All Go packages passed before the final staging-cache-reset
line; build passed afterward. Native UI validation is in progress.

Only test server15011 was restarted; original snapshot world files preserved.
New server PID252713, binary runtime/mpbt3025-presence, cloned world
runtime/world-test-15011-presence.json, log presence-fix.log, traces
runtime/traces/presence-fix. Clients arrival-a252719 and
arrival-b252720 are starting. No other game clients were running at launch.
Next finish native startup for Subvoltic/BetaPilot, keep one staged while
other enters mission to inspect counts/seat preview; join both for headings,
then verify leave/disconnect removes the correct pilot. Combat0x81 remains
unimplemented; preserve the prior loading-splash evidence.

Native follow-ups: the first live staging test exposed reversed count
columns: Alpha's preview entry appeared in the Beta count column. Native
row renderer0x45c6f4 reads defender count/open seats for Alpha;0x45c781
reads attacker fields for Beta. Corrected the RoomInfo builder's mapping
(side0=defender/Alpha, side1=attacker/Beta); the codec field order itself was
already correct. Screenshot presence-final-b-alpha-one.png now confirms
Alpha1/4, Beta0/4 with Subvoltic in the Alpha preview.

Native Back emits JOIN_ROOM(parent city121001/local), not EXIT_ROOM.
Implemented that branch; confirmed it returns to staging and broadcasts
both counts back to0/4 (presence-final-{a-back-to-staging,b-after-peer-back}).
Returning client's cached peer was missing despite0x94 data refresh: roster
membership requires0x2b ARRIVED after RoomUpdate clears the old room.
Added ARRIVED for every peer during room-roster rebuild and for live mission
arrivals. Existing room-to-room departure and disconnect filtering remain.
Native disconnect was confirmed on the first build: killing only A removed
Subvoltic from B's roster/preview, cleared the stored seat and pilot RoomID,
and restored0/4 counts (presence-fix-b-after-peer-disconnect.png).

All Go packages passed after count and Back repairs and roster-message
repairs; additional cached-peer roster regression passed afterward.
Latest live validation stack: server257137 at15011, arrival-a/Subvoltic
PID257138, arrival-b/BetaPilot257143. Both clients starting; trace directory
presence-roster; logs presence-roster.log and presence-roster-{a,b}.log.
World remains world-test-15011-presence.json. Finish startup and verify
roster rebuild, leave/rejoin, and cross-room isolation before reporting all
native behavior fixed. Combat research advanced too: see
research/notes/combat-command-stream.md for three recovered command types and
native PrepareForCombat/ReadyForCombat visitor routing.

Correction to the cached-peer hypothesis: adding0x2b alone did NOT repopulate
A's staging roster on Back; screenshot roster-a-return-with-peers.png still
shows only self. Recovered the actual full roster response UPDATE_OCCUPANTS
0x3b, descriptor0x592e08, writer0x5069d0: requestStatusu32, full BTRoomInfo,
PlayerInfoList (writer0x512250: uint16 count then BTPlayerInfo records).
Added this snapshot immediately after occupants-follow RoomUpdate for both
staging and mission entry. All Go packages passed and binary rebuilt.
Current native snapshot validation stack: server261632 on15011, A261636,
B261639; traces presence-snapshot, logs presence-snapshot.log. Startup just
launched. Source UpdateOccupants and peer-arrival additions must receive
live validation; do not claim cached-peer roster fixed before observing it.

Live snapshot result:0x3b is decoded without disconnect and process-memory
inspection verifies TWO cached player records (Subvoltic and BetaPilot) in
A's native room cache. Its lobby notification guard is0. Yet returning from
a mission that contained only A still displays only A in the lower staging
roster. Fresh A repeated the issue, so the earlier duplicate-callsign test
was not the root cause. B returning from a two-player mission showed both
players correctly. Both mission headings and both players' mission rows are
correct (snapshot-a-both-mission.png); B leaving clears its seat and removes
it from A's mission roster (snapshot-a-peer-left-mission.png). B staging
counts correctly show Alpha1/4 and Beta0/4 with Subvoltic in Alpha preview
(snapshot-b-staging-counts.png).

Current experiment reorders the single full0x3b roster snapshot AFTER the
individual occupant cache/arrival messages. No timer/delay workaround added.
All Go packages passed; binary rebuilt. Latest stack at15011: server274758,
A274763/window0x4e0000f, B274765/window0x440000f; traces/logs
presence-final-order. Both startup screens currently loading. Same world
world-test-15011-presence.json and same arrival prefixes. Confirm window
PIDs before input: Wine reused/swapped window IDs during earlier restarts.

Startup helper now re-reads the window origin after focusing before each
click; Wine desktop placement can change while startup runs. It can still
miss an OK/Go To Planet click while the UI becomes interactive: inspect the
screen and send one verified local-coordinate click with operate-solaris-client.py
instead of retyping the call sign. Keep at most two actual game processes.

Additional follow-up found during repeated leave/rejoin: clearing a room seat
also drops its MechID8 selection, but the client may retain that own selection
and not resend the automatic selection packet on rejoin. Some rejoined rows
show Any Mech. Preserve/reapply the pilot's selected native Mech across room
changes before the next combat-initialization trial. No new implementation
for this follow-up yet.

Ordering trial did not fix the solo-mission return display. More useful
read-only memory evidence: after Back, native CombatManager at0x1743e30
still reports MechID8, own seat1, state0, guard0, while the server pilot is
already in staging. The root0x3e room update does not update the combat
manager's local seat. The leaving client never received the changed old
mission configuration; only remaining room peers received that refresh.

Added the changed old mission0x8b snapshot to the DEPARTING client BEFORE
its staging RoomUpdate. The caller already removed that pilot from model
seats, so this snapshot should set its native own seat to0. This also
explains the missing automatic Mech resend on rejoin: own seat stayed1
across the nominal leave/rejoin instead of transitioning0->1. Added a
regression for the old-mission removal preceding the new-room update; all
Go packages passed and server rebuilt. No timers or forced UI refreshes.

Latest controlled stack: server278288 at15011, arrival-a278289,
arrival-b278294. Log/trace label presence-seat-clear. Both clients currently
starting. Inspect XIDs/PIDs again before input. World file unchanged:
world-test-15011-presence.json. Next verify native own seat0 and both pilots
visible on solo Back, then verify automatic MechID8 on rejoin.

Live verification after the departing-client old-room update: restarted two
clients in the existing arrival prefixes, with Subvoltic and BetaPilot, then
checked the same Solaris mission. While only Subvoltic occupied Mission 1,
BetaPilot's staging list showed Alpha1/4 and Beta0/4, with Subvoltic alone in
the Alpha preview (`client/.runtime/captures/resume-b-one-in-mission.png`).
Subvoltic's Back returned both pilots to staging, cleared Mission 1 to0/4,
and displayed both players in the Solaris roster
(`resume-a-back-roster.png`). After rejoining, both clients showed the expected
Lance Alpha and Lance Beta headings, with Subvoltic/COM-4D in Alpha and
BetaPilot/COM-4D in Beta; the mission occupant count was2
(`resume-a-two-in-mission.png`, `resume-b-two-in-mission.png`). This confirms
the seat-clear refresh also restored the returning client's mission row and
Mech selection. Both in-memory managers retain MechID8 and nonzero native seat state (raw
seat words5 and1); the visible rows identify each pilot under the expected
team heading.

The previous clients had exited between turns. They were relaunched in the
same two prefixes and connected to the existing server at127.0.0.1:15011.
Current test server remains PID278288; game processes305181/305184. Do not
claim combat playable: combat launch still stops at the loading splash because
native combat command0x81 has not been reconstructed.

2026-10-22 continued: added executable native combat fixtures, verified 0x81
framing/checksum/NUL semantics, and recovered the stock COM-4D bootstrap from
native FakeStart/FakeACat. Source now includes combat_stream.go,
combat_bootstrap.go, fixture-based tests, and an opt-in
-experimental-combat-init launch path. All Go packages passed and built
runtime/mpbt3025-native-combat. Full findings/repro in combat-command-stream.md.

Previous live window/prefix mapping correction: Subvoltic was window4c/PID305184
in arrival-b; BetaPilot was window4a/PID305181 in arrival-a. Thus native own
seat1 belonged to Subvoltic, own seat5 to BetaPilot, consistent with shell
Alpha/Beta. The prefix names alone do not identify which pilot was typed in.

Current controlled combat trial stack: server309545 at127.0.0.1:15011,
world-test-15011-native-combat.json (clone preserves the prior presence world),
log native-bootstrap.log, trace directory native-bootstrap. New game PIDs
309590 and309592; window4c maps to309590, window4a maps to309592. Both startup
screens loading. Confirm PID->prefix and typed identity before further input.
The new initialization is enabled only on this test server. Other listeners
15004/15008 remain untouched. At most two actual BT3025 clients are running.


2026-10-22 model-switch checkpoint (supersedes earlier stack/status):
Native combat bootstrap now renders BOTH desert cockpits with complete
Commando COM-4D weapons/HUD. Evidence: client/.runtime/captures/
native3-after-setup-a.png and native3-after-setup-b.png. That stage3 run
correctly used EXACT Mech count2 and deferred Ready; both remained at Stand By
and emitted native position commands. Immediate Ready had previously crashed
K2D.dll through ReadyForCombat/CHUDBogie::AddBogie because HUD was not initialized.

Current code now waits for validated native position command0x27 (37 bytes)
from both expected pilots before broadcasting Ready once. Full Go suite passes,
including native DLL fixture comparisons and the two-client barrier regression.
The fixture runner scripts/native-combat-fixture.py executes the original
x86 writers offline; no debugger attachment or client modification.
Details and corrected count semantics: research/notes/combat-command-stream.md.

Current controlled stack: server311939, binaryruntime/mpbt3025-native-combat-barrier,
127.0.0.1:15011, world-test-15011-native-combat.json,
logruntime/logs/native-bootstrap-barrier.log,
traceruntime/traces/native-bootstrap-barrier.
Only two game processes:311942 arrival-a / window0x4a0000f / Subvoltic;
311945 arrival-b / window0x4e0000f / BetaPilot. Wine desktop capture targets
0x3c00007 and0x4000007 respectively. Both joined Mission1 and readied;
countdown started13:54:58 server-log time. Live release result follows below.
Other listeners15004/15008 untouched. No Git checkout. Never start a third game.
Movement relay, damage, and authoritative combat remain outstanding.

LIVE BARRIER SUCCESS:13:55:54 both native setup packets sent;13:55:58 log
"All 2 native clients in mission 1 prepared; releasing combat". Both clients
remain alive, no Program Error dialog. Direct Wine-desktop captures show
Stand By GONE, complete arena/HUD, match clocks4:45/4:46, correct pilot names.
Evidence: client/.runtime/captures/barrier-released-a.png and
barrier-released-b.png. No movement keys tested in this final trial.
Leave this exact two-client stack running for continuation.

Next useful work: reconstruct native server position command wire0x62 ('b'),
then relay authenticated peer movement scoped to the launched mission.
Registry key0x41 factory/vtable0x6908e284, writer0x6906f350,
decoder0x6906f3d0, visitor0x69073640. Payload after type: entity byte,
signedX4, signedY4, unsigned3, four signed3 values (25 total command bytes).
Client position command wire0x27 registry key6 / vtable0x6908e2bc,
decoder0x6906f7a0: signedX4, signedY4, unsigned3, seven signed3,
unsigned4 (37 total). Candidate peer relay: wire0x62 + authenticated compact
entity byte + client bytes[1:24]. MUST validate with native fixture before use.
Movement/damage remain unimplemented; current success is combat entry only.


2026-10-22 continuation: implemented and LIVE VERIFIED native peer position relay.
Native fixture tool now executes BOTH original position writers:
client key6/0x6906f700, server key0x41/0x6906f350. The server command is
wire 'b' + authenticated entity ID digit + the shared first23 data digits
of the client's37-byte wire 0x27 command. Native fixture bytes match the
Go mapper exactly and native receiver preprocessing round trips both frames.
The server starts relay only after both clients have prepared and Ready has
been sent; peers must belong to the same launched room. It ignores invalid
checksums and only relays exact position commands. New regression covers
before-Ready, other-room, spoofed membership and bad-checksum cases.
All server Go packages passed.

Controlled relay stack (supersedes previous stack): server PID313495 at15011,
binary runtime/mpbt3025-native-combat-relay,
world runtime/world-test-15011-combat-relay.json,
log runtime/logs/native-combat-relay.log,
traces runtime/traces/native-combat-relay.
Client PIDs313545/313546 in existing arrival-a/b prefixes, shell windows
0x4a0000f/0x4e0000f, combat windows0x4a0000b/0x4e0000b, Wine desktops
0x3c00007/0x4000007. Names Subvoltic/BetaPilot, respectively.
They readied in Mission1 at14:04:07/14:04:10. Native setup at14:05:07,
both prepared and Ready release at14:05:11. Server logs continuous
42-byte sends after incoming53-byte position packets. Both cockpits and match
clocks live, no Program Error. Captures:
client/.runtime/captures/relay-live-{a,b}.png.
Held W for2s on Subvoltic's combat window with focus verified; its HUD showed
20kph and changed ground view, while BetaPilot's peer indicator moved to the
left on its HUD. Stopped with X. Captures:
client/.runtime/captures/relay-after-w-{a,b}.png.
This demonstrates local control and peer position response; no damage/fire
simulation yet. Native position visitor0x69073640 calls0x69045920 to apply
peer state when Reality state=3. Do not start a third BT3025 process.


2026-10-22 fire continuation: live Left Alt fired Subvoltic's selected weapon.
The native client sent one checksum-valid0x81 frame with a37-byte position
command followed by a36-byte fire command (wire0x2b, key0x0a) at14:07:25.
The client fire writer is BTCombat.dll:0x69070c20. Native registry separates
client key0x0a (0x69073e20) from server key0x44 (0x69073e40).
The matching server fire writer0x69070810 emits wire0x65 ('e'), then a sender
entityID digit, then client fire bytes[1:35], dropping the final client byte.
Offline native writer fixtures prove this conversion exactly, including the
native receive preprocessing. Added CombatPeerFire and handling of a combined
position/fire frame; the relay sends both commands only to other rostered
clients in the same active mission. Full Go suite passes.

Controlled fire trial (supersedes relay stack): server315129 at15011, binary
runtime/mpbt3025-native-combat-fire, world
runtime/world-test-15011-combat-fire.json,
log runtime/logs/native-combat-fire.log,
traces runtime/traces/native-combat-fire.
Only game PIDs315189/315190; shell windows0x4a0000f/0x4c0000f,
combat windows0x4a0000b/0x4c0000b, Wine desktops0x3c00007/0x4000007.
Subvoltic/BetaPilot respectively. Both ready14:11:40/14:11:44;
setup14:12:41; Ready release14:12:44. At14:13:05, held Left Alt0.5s
on focused Subvoltic; server received length89 Aries frame (combined
position+fire) and sent translated fire length53 to BetaPilot.
Both clients remained alive and in combat, no Program Error.
Capture client/.runtime/captures/fire-relay-{a,b}.png shows active HUDs;
Subvoltic's selected weapon advanced from M Laser to SRM-6 after firing.
No peer projectile or damage confirmed visually in this shot, as both pilots
were not in clear mutual sight. Do NOT claim successful hit/damage yet.

Native receive visitor0x69073770 for server fire commands requires Reality
state3, ignores shooter equal to localentityID, and has a branch comparing
(target byte at command+0x32)-1 to localentityID before effect/damage work.
This suggests that a correctly targeted client fire report can trigger native
hit behavior, but needs a controlled hit trial and server-side validation.
The server currently relays the authenticated sender ID and client-reported
weapon/target data; no authoritative hit validation or victory/results loop.


## Continuation after repository move — 2026-10-23

The active server checkout is now `/home/sebulba/dev/MPBT3025-Server`, with
source in `src/` and overlay tests in `tests/`. See the
[October 23 checkpoint](solaris-session-2026-10-23.md) for the current city-roster
correction, tests, native verification and remaining work. Earlier runtime paths
and process IDs in this historical note are not current.
