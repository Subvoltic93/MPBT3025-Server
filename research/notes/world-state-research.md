# Multiplayer BattleTech 3025 world state

Historical local paths in this document are relative to the original workspace root. Original client assets, client-derived research, and visual evidence remain in the separate workspace-root `client/` directory and are not included in this server project.

## What the world server had to own

The Player's Guide describes one shared campaign state with several views over
it, rather than a collection of unrelated lobbies:

```text
Pilot/account
  ├─ rank, House, credits, owned 'Mechs, combat record
  └─ current location
       └─ Inner Sphere map: 120 territories, five House control values
            └─ planet: strategic sectors and current 'Mech occupancy
                 └─ staging area: active mission rooms and seat occupancy
                      └─ mission: ready/countdown, combat, results
                           ├─ pilot score and progression
                           └─ House control changes on the planet
```

The guide says five Houses contest 120 territories. Each planet has a current
controlling House and five percentage-control values. Ownership and adjacency
determine whether a House may attack or defend there. Strategic importance also
sets economy value and the maximum 'Mech class. The map is described as updating
in real time when control changes. These facts make the 0xa7/0xa8 planet-state
exchange a world snapshot, not a cosmetic map refresh.

Planet entry is hierarchical. **Go To Planet** presents strategic sectors and
their current 'Mech occupancy; selecting a non-full sector and choosing **Go To
Sector** enters its staging area. A staging area shows mission rooms, attacker
and defender seat counts, room status, participating pilots, and the planet's
five control percentages. Ready state and a countdown control mission launch.
Battle results update both pilot records and planetary control. The guide says
salary also depends on the economy of the planets owned by a pilot's House.

That is the practical server-state model to build toward: authoritative pilots,
Houses, planet/control/economy state, sector occupancy, mission rooms/seats,
and battle results need to be connected. A room-list reply alone cannot recreate
the normal game.

## New-pilot progression matters

Solaris is a separate, neutral training planet. The guide describes training and
battles there before House selection; reaching 1,200 rank points awards the
Solaris Graduation Medal and permits a House choice. It also says the My 'Mechs
and My Stats controls remain gray during this phase. This matches the user's
observation that some top-bar controls were unavailable after Basic Training;
that alone is not evidence that the map's campaign state is broken. Basic
Training did not grant the documented graduation medal or rank points.

For the current bootstrap, allow the new pilot to enter Solaris without a House.
Do not fabricate House membership or Inner Sphere territory ownership to make
the buttons active. The local client has already rendered a map, but it has not
shown that it received the live territory state.

## Wire findings and current implementation

Local binary inspection identifies these relevant messages:

- `JOIN_PLANET` (0x91): the client serializer labels its second field
  `m_password`; the observed Solaris request contains planet ID 121 and an
  empty password. The corrected class-ID mapping is `COMBAT_SUMMARY` (0x90),
  `JOIN_PLANET` (0x91), and `JOIN_PLANET_FAILED` (0x92). A prior eight-zero-byte
  ID0x90 reply crashed the client, but it was a malformed combat summary. A
  zero-result ID0x92 produced a failure dialog and did not transition the map;
  it is not a successful join response.
- `REQUEST_PLANET_STATES` (0xa7) carries a uint32 planet ID. Its matching
  `PLANET_STATE_LIST_UPDATE` (0xa8) contains a counted state list, sequence
  number, and seconds-until-shift. An empty list was accepted but removed the
  map's faction colors, so the server does not send it as a placeholder.
- `REQUEST_LOBBY_ROOMS` (0xad) carries a `m_planetID` field. The field name is
  visible in the client serializer at `BT3025.exe:0x521f80`; the earlier Go
  parser incorrectly treated the four bytes as an empty list count. The parser
  now reads the planet ID and can reply with the known empty `READY_ROOM_LIST`
  (0x97).

The 11:29 live trace records the client sending `JOIN_PLANET` (0x91) for
Solaris (ID 121), then the server sending an empty 0x97 room list. The client
stayed on the map, continued sending 0xa7 planet-state requests, and never sent
0xad. So 0x97 is not a sufficient join transition in this flow. That early
reply has been removed from the Go handler; 0x97 is now sent only in response
to the explicit 0xad request. In the later 12:23 experiment, the VCR-derived
0xa8 reply was followed by a client runtime abort. In the current no-flag run,
the user completed Basic Training, selected Solaris, and clicked **Go To
Planet**. The trace contains ID0x91 for Solaris followed by repeated ID0xa7
requests, with no server reply to the join and no ID0xa8 update. The 12:29
screenshot still shows the Inner Sphere map with Solaris selected and **Go To
Planet** active; its **Communications** and **MechWarriors on Planet** panels
are empty. The manual says the next view should list strategic sectors and
their 'Mech occupancy, so the client has not visibly reached the sector list
or staging-area lobby.

The 0xa8 element's wire shape is now recovered, but its field meanings and
valid starting control values are still unknown. Keep the empty 0xa8 update
disabled. The client cache `BT3025.vcr`
contains a big-endian count of 121 at file offset `0x3848`, followed by records
with planet IDs 1–121, including Solaris (121), and then House records. Planet
names and descriptions are readable. Further disassembly found a `PlanetInfo`
serializer at `BT3025.exe:0x515ca0`, decoder at `0x515d60`, and field formatter
at `0x515e30`. Those codecs show that the class is serializable, but no direct
reference ties it to the ID0xa8 list element type. The VCR record table is
therefore still only a candidate source, and is not known to contain live
ownership or House control percentages.

The SDS Appendix A provides separate evidence for the **initial** owner of
many preserved planets: it groups worlds by starting House affiliation and
marks Solaris neutral. Matching those lists to the 120 non-Solaris cache names
gives 108 unique House assignments. Eleven names are absent and Winfield is
listed under two Houses, so those twelve entries remain unassigned. The
prototype world now uses the 108 unique initial assignments and represents
each as 100% control by that House. This is a static starting seed, not live
campaign data. The `0xa8` wire fields are still unknown, so this ownership has
not yet been projected into client map colors.

The clean baseline at 14:10 provides a useful crash comparison. It omits both
the session-specific 0x3c/0x3d records previously sent after call-sign creation
and the experimental 0xa8 snapshot. The client reaches the same Solaris
**Go To Planet** screen without visual corruption or a crash, then sends 0x91
and continues 0xa7 polling. This narrows the crash to the prior experiment's
added messages collectively; it does not isolate either message. The join
request still produces no visible transition or sector list.

The 14:17 isolated run narrows this further. With both post-create records
disabled, the server sent only the compact 202-byte 0xa8 after the client's
0xa7 request. The user saw the Microsoft Visual C++ Runtime Library dialog
reporting abnormal program termination while the map and blank call-sign pane
were still visible; the trace has no later client PDU. The 0xa8 candidate alone
is therefore unsafe. This rules out the additional 0x3c/0x3d records as a
necessary cause of that crash.

The raw VCR record also does not match the client's `PlanetInfo` network
serializer. The Solaris cache record begins with ID 121, a length-prefixed
`Solaris` name, and type byte 2. After that byte, the cache contains the two
lengths `00000007 00000085` before the corresponding text. The network
serializer at `BT3025.exe:0x515ca0` instead writes each string length directly
before its text. If interpreted as a network `PlanetInfo`, the cache's first
post-type string would consume `00000085Sol` instead of `Solaris`. The A8 list
element type is still unconfirmed, but the VCR bytes cannot be copied directly
as a network record. Keep both VCR flags off.

The Go server has an opt-in experiment, `-planet-vcr <path>`. It accepts only
the preserved VCR hash, extracts the 121-record planet table, then appends
sequence 0 and seconds-until-shift 0 to make an ID0xa8 candidate. This avoids
guessing individual field values and makes the seed explicit. The resulting
PDU is about 28 KB, larger than the 16 KB buffer advertised in `HOST_ONLINE`.
Factory case 0x47 at `BT3025.exe:0x4f69a3` constructs the split-message class;
its serializer at `0x5027a0` writes a one-byte `m_lastInSet`, uint32
`m_dataLen`, then that many raw bytes. The Go server now places the complete
encoded ID0xa8 stream (including its six-byte header) into ordered ID0x47
fragments, each in a separate Aries data frame within the advertised size, and
marks only the final fragment. The client-side reassembly path and VCR/list
record compatibility were both unknown, so this was tested once with the
client. Trace `session-1792437754057918789.log` records the 0xa7 request for
planet 0 followed by two split frames for the candidate. The client then
showed a Microsoft Visual C++ Runtime Library “abnormal program termination”
dialog before **Go To Planet** was clicked; no further client packet appears in
the trace. This confirms the candidate delivery is unsafe, but does not isolate
whether split-buffer handling or the guessed state-list data caused the abort.
Keep the option disabled until the receive/reassembly path and list element
layout are established independently. A clean run without the option reached
the call-sign form without sending this large update. The VCR remains a local
cache, not a known authoritative campaign snapshot.

## Prototype campaign state in the Go server

The current server keeps gameplay rules in `src/state`, separate from Aries
and PDU encoding. A JSON world file stores pilots, Houses, planet control,
neighbor links, sectors, Solaris cities, mission rooms and seats, countdowns,
mech purchases, salary claims, and applied battle IDs. Pilot creation is
idempotent across reconnects. Cadets can join Solaris, earn prototype rank
points from a completed battle, graduate at the documented 1,200-point mark,
and select a House afterward. The model also includes sector eligibility,
four-seat lances, solo AI opponents, basic C-bill rewards, and idempotent
territory changes for campaign results.

The `data` package uses generic planet labels and synthetic ownership by
default. Optional operator-supplied seed tables are loaded from the directory
selected by `MPBT3025_SEED_DIR`; extracted client cache data is not embedded in
this project. Other world values remain provisional: ownership is split among
the five Houses, neighbors form a ring, each Inner Sphere world has three
generic sectors, and room/cash/score values are placeholders. The manual
confirms the mechanics, not those values. Operators can edit
`runtime/world.json` after its first creation; deleting it rebuilds the
prototype seed.

The `-planet-vcr-solaris` experiment sends only the 186-byte Solaris cache
record in an ID0xa8 update, so it stays within the client's 16 KB buffer and
does not use split-buffer framing. Its isolated live trial still terminated
the client. It is retained only as a clearly labeled unsafe diagnostic; do not
use either VCR flag in normal client runs.

The current server applies `JOIN_PLANET` to persisted pilot location but does
not yet know the client success event that enters Solaris's city-lobby screen.
`JOIN_PLANET_FAILED` is now encoded as ID0x92 with the recovered base result
and string fields. The full lobby PDU formats and the successful join sequence
remain the next protocol work.

In the 14:30 local player-only refresh trial, the client accepted the `0x88`
create completion and the session's `0x3c` player record, then reached training
and sent `0x91` for Solaris. The screenshot showed a stable map, but its Pilot
Information panel showed unavailable location and statistics because the
post-creation `0x3d` profile refresh was omitted. The map did not transition;
the server trace contains no `0xad` lobby-room request. Compared with the
previous no-`0xa8` run that sent both post-creation records and showed clipped
planet text, this is evidence that the profile refresh may be involved in that
rendering problem, not proof of the cause. The exact trace and screenshot are
listed in `bootstrap-handoff.md`.

The next populated-room experiment exposed an earlier serializer mistake. A
one-entry `0x97 READY_ROOM_LIST` candidate was sent after the Solaris `0x91`
request; the client terminated immediately, with a runtime error visible in
[`evidence/solaris-one-room-list-crash.png`](../../../../MPBT3025/client/.runtime/captures/server-evidence/solaris-one-room-list-crash.png).
The candidate treated the message as a count followed by `BTRoomInfo`. The
binary serializer at `BT3025.exe:0x5287a0` instead emits a container at object
offset `+0x08`, a uint32 at `+0x28`, then two containers for each node in an
auxiliary linked structure at `+0x20`. The matching decoder at `0x528810`
shows those same outer fields. The entry types and semantics remain unknown,
so the live candidate has been removed. This crash establishes that the
candidate was unsafe; because its bytes violate the recovered outer layout, it
does not establish that a correct populated room list would crash the client.

Static follow-up decoded the `+0x08` container's callbacks: it serializes a
count and 4-byte elements, not inline `BTRoomInfo` records. The `+0x20` member
is a counted ordered tree: `+0x28` is its entry count, followed by each
entry's custom string-like key and a 32-bit `m_value` wrapper. On the wire, a
key contains a uint32 discriminator followed by a length-prefixed string; the
comparator uses the string's byte content and length, then that discriminator.
Its meaning is not named. The tree's purpose and the list elements' meanings
are still unknown. `BTRoomInfo` has a separate codec. The
candidate's leading `1` may have been a valid list count, but its following
room-record bytes and missing map count/entries made the response invalid.
See `bootstrap-handoff.md` for the recovered codec details.

In the clean 14:42–14:47 baseline, the user clicked **Go To Planet** again
after the populated-list sender was removed. The client remained stable on the
map; the server trace contains `0x91` for Solaris, repeated `0xa7` state polls,
and heartbeats, but no server reply to the join and no `0xad` lobby-room
request. The updated screenshot and trace are recorded in
`bootstrap-handoff.md`. The immediate milestone is therefore a valid
map-to-planet/sector transition, before any populated lobby room list.

## Internet sources checked

- The [MPBT:3025 Player's Guide v1.4.5](https://device.report/m/9776385816d7385322976ddba35c95e5bb7b7f7fc15658ebbc6d92bf324cdbc1.pdf)
  is the primary product manual; a local copy is kept at
  [`../../reference/manual/Multiplayer-Battletech-3025_Manual_Win_EN.pdf`](../../reference/manual/Multiplayer-Battletech-3025_Manual_Win_EN.pdf). Pages 7–21 describe Solaris, the map, staging areas, mission rooms, launch, and
  control updates; pages 24–28 describe pilot rank and salary. The relevant
  sequence is especially clear on pages 13–16 and 19–21.
- Andrew Hassell's [2024 emulation demonstration](https://www.youtube.com/watch?v=Yiv9tl4QD7Q)
  is evidence that someone recreated enough of the server to run the client. It
  does not publish the protocol or establish that the full campaign was
  implemented. I did not find a verified public source repository for that
  emulator in this search.
- The public [kenhuman/mpbt-server repository](https://github.com/kenhuman/mpbt-server)
  documents a separate Multiplayer BattleTech: Solaris server emulator for
  the 1996 `MPBTWIN.EXE` client. Its README says that game used Kesmai's ARIES
  engine, also used by Air Warrior and Legends of Kesmai, and describes a
  lobby service handing clients to a separate world server, with persistent
  accounts, room presence, world travel, and arenas. This supports keeping
  accounts, world state, lobby state, and combat as clear service modules.
  The Solaris 1996 client's inner protocol is different (including a custom
  escape frame and command table), so its opcodes and payloads are not evidence
  for BT3025. Treat it as architecture research; do not copy code without
  addressing its AGPL-3.0 license.
- A contemporaneous [Argent Fire interview with the IS3025 project](https://cf.sarna.net/docs/newsletters/argentfire/afv1i3.pdf)
  says Project Jenner began as reverse engineering of the EA server and then
  pivoted to starting over. That is historical context, not a protocol spec for
  the released beta client.

The manual establishes game behavior, not binary packet layouts. For wire
formats, the local executable serializers and observed traces remain the direct
evidence; keep those claims separate from the manual-derived gameplay model.

## 0xa8 entry wire shape (16:49 local)

The `PLANET_STATE_LIST_UPDATE` decoder at `BT3025.exe:0x523a10` delegates its
container member to a counted decoder. That decoder (`0x4e58a0`)
constructs a `0x35`-byte entry at `0x517c20` and invokes its decoder at
`0x517d40`; serialization uses `0x517ce0`. The entry codec writes two
`uint32` values, a counted `uint16` array, a counted byte array, and a trailing
byte. The parent PDU adds a `uint32` sequence and `uint16`
seconds-until-shift. The remaining field meanings are not named by this codec,
so the Go type uses `Key` for the first scalar (identified as the collection
lookup key) and neutral labels (`ValueB`, `Values16`, `Values8`, `Flag`) for
the rest.

The opt-in `-planet-state-schema-probe` sends one entry with `Key=121` and
the remaining fields zero. In the live trial, the client decoded this
33-byte update and stayed open through a later Solaris join, but remained on
the map. This establishes that this one-entry numeric packet is safe to decode;
it does not establish the remaining field semantics or valid campaign state.
The probe stays off by default. Exact-term Internet searches found no BT3025
protocol document; the separate 1996 Solaris server and other Aries-era games
remain architectural references only, because their inner wire protocols
differ.

## Malformed string-shaped probe result (14:56–14:58 local)

An early one-entry string-shaped `0xa8` candidate followed the user's Solaris
`0x91` join and the client's `0xa7` request for planet 0. The UI remained on
the Inner Sphere map; after the user clicked the upper-left **Game** menu, Wine
showed an abnormal-termination dialog. Later serializer inspection showed this
candidate was malformed: the element begins with two numeric fields, so the
string bytes shifted the arrays and later fields. The screen capture shows
black House territories. The Wine log reports failure to allocate
`0x40010000` bytes (1 GiB); the process and socket were still alive at the time
of capture, and the trace contains no later client PDU. This failure does not
establish how a correctly encoded `0xa8` is handled. Keep experimental state
packets disabled outside controlled trials. See the dated handoff entry and
capture for the full trace and screenshot.

The immediate clean relaunch omitted the probe and included the required
`aol:` startup argument. It completed Aries login/config synchronization and
showed the intro **Next** page, confirming upload completion without exercising
the still-unresolved lobby transition.

## 0xa8 key lookup follow-up

The counted-container decoder at `BT3025.exe:0x4e58a0` inserts decoded
`0xa8` records into a keyed tree using the first serialized `uint32` (the
entry object's value at `+0x8`). The consumer at `0x460ee4` looks up an entry
with a `uint32` taken from the current world object at `+0xa4`; the tree lookup
comparator is at `0x42fcd0`. This supports describing the first wire value as
the record's lookup key. The binary does not name that key, and the
`+0xa4` world-object field has not been independently identified, so “planet
ID” remains a strong contextual inference rather than a recovered field name.
The other entry fields remain unresolved. A 1–121-key probe can test list
cardinality and key presence, but cannot be treated as world data.

## October 20 baseline captures

The three newest server traces—[`session-1792508819539932789.log`](../../runtime/traces/session-1792508819539932789.log),
[`session-1792510229117640325.log`](../../runtime/traces/session-1792510229117640325.log),
and [`session-1792511031958442614.log`](../../runtime/traces/session-1792511031958442614.log)—show startup, call-sign creation, heartbeats, and the global `0xa7` poll. None contains `JOIN_PLANET` (`0x91`) or a server `0xa8` response. Those captures did not answer the cardinality trial because the client had not reached the Solaris join. The later 11:25 run exercised the 121-entry probe; see the latest entry in `bootstrap-handoff.md` for its unchanged-map result.

## Owner-colored map reference and wire-field investigation (2026-10-20)

The user supplied a fresh-map screenshot (`p2t226-b-fresh-map.webp`) showing
the Inner Sphere territories filled with their controlling House colors. The
legend maps Davion to yellow, Kurita to red, Liao to green, Marik to magenta,
and Steiner to blue; Solaris is neutral. This confirms the desired map output.

The preserved client VCR HouseInfo table assigns IDs 0 to `none` and 1–5 to
Davion, Kurita, Liao, Marik, and Steiner. A first opt-in projection placed
House IDs in the second scalar of each `0xa8` element. Closer disassembly of
the map consumer at `0x43bd70` showed that scalar is used to look up a map
object at `0x479290` and update it at `0x479230`/`0x478df0`. Repeating just
five House IDs would alias many of the 121 objects. The unrun projection was
removed. The actual owner field has not been identified, and the all-zero
probe does not provide ownership colors.

## Complete initial ownership from PlanetInfo cache (2026-10-20)

The client cache's 121 `PlanetInfo` records contain a House field. The client
serializer at `0x515cc9`–`0x515cd3` writes the field after ID, name, and type;
the formatter at `0x515f56` labels it `m_house`. Matching each planet's ID and
name in the hash-checked VCR gives 25 each Davion, Kurita, Marik, and Steiner,
20 Liao, and Solaris with special House value 7. This source fills all twelve
SDS gaps and assigns Dixie to Steiner, where the SDS-derived seed had Marik.
The server's version 2 initial seed now follows the cache, with Solaris neutral
in the campaign model. Safe migration of untouched version 0 and 1 worlds is
implemented. This is server world data, not yet a decoded `0xa8` owner field.

A controlled relaunch kept the existing server and repeated one Solaris join.
The `0xa8` full zero-record probe changed black map interiors to gray cells,
but did not produce a `0xad` room request or a planet-screen transition. See
`bootstrap-handoff.md` and trace `session-1792522066610043032.log`.
