# Solaris continuation — 2026-10-23

## Workspace recovery checkpoint

The server repository moved to `/home/sebulba/dev/MPBT3025-Server`, origin
`https://github.com/Subvoltic93/MPBT3025-Server.git`, branch `main`.
The separate local client stays under `/home/sebulba/MPBT3025/client`.
Source is in `src/`; tests are `.go.txt` files under `tests/`, run with
`scripts/test.sh`. Do not use the old server paths in earlier session notes.

At resume, server PID 375803 was listening on 15011 with the native combat/fire
build. No clients were running. The October 22 handoff confirms mission roster,
seat headings, ready/countdown, native combat entry, position relay and fire
relay trials. Hit/damage and match results remain unverified.

## City roster correction checkpoint

Solaris staging RoomIDs are city-specific, but `publishPlanetJoin` previously
selected peers by planet alone. This mixed other cities into the roster and
occupancy, and broadcast arrivals/departures across city boundaries. Restricted
Solaris presence areas to planet plus city, keeping seated pilots visible to
their own city's staging viewers and preserving mission-room filtering.
The new cross-city regression failed before the correction and passes afterward.

The full suite initially exposed a stale test that assumed the original private
campaign ownership seed. It now assigns representative House owners explicitly
and verifies their protocol IDs independently of optional seed files. Full suite
passes using Go 1.24.13 and the test overlay. Built `runtime/mpbt3025-city-rosters`.

Next: restart only the isolated 15011 test server with a cloned saved world;
start at most two clients in the arrival-a/b prefixes from their installation
working directories. Use `client/scripts/operate-solaris-client.py` for bounded,
focused input/captures. Avoid VCR flags and populated 0x97 responses.

## Live checkpoint — city roster build

Test server PID 386616 uses `runtime/world-test-15011-city-rosters.json`, cloned
from the previous combat/fire world; traces/logs use `city-rosters`. Clients
386617 (arrival-a, Subvoltic, shell 0x440000f) and 386981 (arrival-b, BetaPilot,
shell 0x4c0000f) are the only two clients. Both run from their installed game
directories at nice priority 10. Startup rendered normally.

Confirmed: empty list 0/4 per side; one Alpha pilot shows Alpha 1/4, Beta 0/4
with matching preview; Back clears both mission counts; the settled returning
staging roster includes both pilots. Both clients then joined Mission 1 with
correct Alpha/Beta headings, both COM-4D seats and roster count 2. All message
`RosterAll23` from BetaPilot displayed on Subvoltic. Both Ready clicks produced
visible READY states and a 60-second launch countdown. Captures are under
`runtime/captures/city-rosters-*.png` (local, ignored).

## Final live result and runtime checkpoint

Both Not Ready clicks returned the seat rows to NOT READY and stopped the
countdown. All (`RosterAll23`) rendered at both clients; Team (`RosterTeam23`)
and House (`RosterHouse23`) rendered only at BetaPilot. The pilots occupied
opposite sides and both persisted House fields were empty, so this confirms
filtering for this case, not assigned-House or same-side delivery.

Readied both again at 09:29:38/09:29:40 local log time. Countdown expired at
09:30:38; at 09:30:42 the server logged that both native clients had prepared
and released combat. Both desktop captures show active COM-4D HUDs, the correct
pilot names, arena, radar and running match clock; no loading dialog remained.
No new hit, damage or result behavior was implemented or claimed in this trial.

Saved before/after captures in `runtime/captures/city-rosters-*.png`, including
`b-alpha-one`, `a-back-settled`, `b-empty-again`, `a-two-mission`, `b-two-mission`,
`a-ready-chat`, `a-cancelled`, `b-channels`, `a-channels-peer`, and both
`a-combat`/`b-combat` images. Captures and packet traces stay local and ignored;
the public repository does not redistribute client art or binaries.

Stopped only the two arrival-prefix Wine servers after capturing evidence.
Confirmed persisted pilot RoomIDs are zero and Mission 1 has no occupied seats.
At this checkpoint no game clients are running. Server PID 386616 remains on
127.0.0.1:15011 using the new binary and cloned trial world. Never run more than
two clients. No unrelated servers were touched; no VCR flags or populated 0x97
responses were used.

## Validation and next work

- `GO=/home/sebulba/.cache/mpbt3025/toolchains/.tools/go1.24.13/bin/go ./scripts/test.sh -count=1`: all packages pass after the city regression and seed-independent ownership test changes.
- Server build and generated documentation pages complete.
- Same-city native checks pass: count/preview, Back/rejoin, full roster, both team headings and Mech selections, chat channel filtering, ready/cancel/countdown and combat entry.
- Cross-city isolation is covered by the new regression. A native trial across cities, reconnect and same-side assignment remains pending.
- Continue combat work with a controlled targeted shot; authoritative hit/damage and match completion remain unfinished.

## Fire reference investigation checkpoint

Continued from the same clean checkout and idle 15011 server. The native client
fire writer 0x69070c20 emits weapon slot then target-plus-one; server writer
0x69070810 inserts shooter before those fields. Visitor 0x69073770 uses server
object +0x31 as weapon slot and +0x32 minus one as target entity. Zero means
untargeted. The previous relay checked base85 digits but allowed arbitrary
weapon/target references through to native object lookups.

Added a fire-selection parser and room validation: stock COM-4D slots 0..2,
and either an untargeted shot or a target in the launched room roster. The
existing authenticated shooter and active-room checks remain. Native fixture
tool now accepts synthetic weapon/target overrides; two targeted fixtures
prove the client and server serializers agree. Full Go suite passes, including
invalid target/weapon and malformed command cases. Per-shot logs identify slot
and target without claiming a hit. Built `runtime/mpbt3025-fire-selection`.

Next live trial uses a cloned world, the same arrival-a/b prefixes, at most
two clients, installed game working directories and nice priority 10. No
VCR flags or populated 0x97 responses. Test targeted firing; do not infer hit
or damage from the target field alone.

## Live fire checkpoint — standalone event found

Fire-validation trial server 393795, clients 393796/393802 (arrival-a/Subvoltic
and arrival-b/BetaPilot), shell windows 0x4a0000f/0x4e0000f. Native combat
windows are 0x4a0000b/0x4e0000b; shell windows become unmapped during combat.
Both prepared and entered combat at 12:29:08 local log time. Focused keyboard
input and desktop captures used only the two existing clients.

At 12:33:10 Subvoltic emitted a standalone 36-byte fire command, weapon slot 0,
no target. The old relay silently dropped it because it only accepted a leading
37-byte position. At 12:34:13 a combined position+fire packet reached the new
reference checks and relayed with slot 2, target -1 (untargeted). This confirms
valid live fire passes reference validation; it does not confirm a hit.

Added complete supported-command batching: optional leading position plus one
or more fire commands, including standalone fire. Unknown/truncated tails reject
the complete batch; a standalone shot cannot release the preparation barrier.
The captured standalone event is stored as a small protocol fixture; integration
coverage verifies active-room-only delivery. Native targeted fixtures and
malformed/unknown weapon/target tests pass with the complete Go suite.

Mouse movement during focus changed the aim in the first trial. Toggled HUD
cursor mode to settle the view, then returned to mouse mode for firing. No
opponent lock, targeted shot or receiving-client damage was established. Keep
these facts distinct from successful valid-fire relay. The clocks expired
without a server-owned result, as expected for the unfinished match loop.

## Fire batching final checkpoint

Stopped only the arrival-a/b Wine servers after recording the first trial.
Both pilots' saved RoomIDs are zero. Rebuilt the final standalone/batch-aware
server and restarted only 15011: PID 395091, binary
`runtime/mpbt3025-fire-selection`, world
`runtime/world-test-15011-fire-selection.json`, log
`runtime/logs/fire-selection-batching.log`, traces
`runtime/traces/fire-selection-batching`. No clients are running. The final
batching change is validated by the captured native event/integration tests;
it has not yet been repeated as a new live standalone-delivery trial.

Captures and keyboard-only aim helper remain local under `runtime/`; no client
screenshots or binaries are committed. Public fixtures contain serializer or
captured protocol bytes only. Full Go suite, server build and site generation
are the completion checks. Continue with controlled targeting/clear sight;
no hit/damage/result implementation is claimed by these changes.

## Controlled spawn investigation checkpoint

Current checkout resumed clean at 36dcd83, server 395091 alone on 15011, no
clients. Recovered FillMech's final initialization at 0x6904207e..0x690420fd:
it computes `pi - atan2(X, -Y)`, scales both signed coordinates by .01,
and calls SetPosition (0x69045920). The pi global 0x690d3f18 is initialized
at 0x69042d70 from acos(-1). The existing X=100/120m, Y=1000m spawns were
almost parallel and only 20m apart, making keyboard-only targeting awkward.

Changed only experimental native launch positions: opposite lances at Y=+/-100m,
seat offsets X=+/-30m per slot, mirrored through the origin. First opponents
start 200m apart and face along the same origin line. Added spawn uniqueness/
side/mirror coverage. Native visibility/target confirmation is still pending;
this is a controlled diagnostic arrangement, not recovered production spawns.

Facing-spawn live trial started: server PID396413, clients arrival-a396414
(shell0x4e0000f/frame0x60d38e), arrival-b396419 (shell0x4a0000f/frame0x60d2e1).
World `runtime/world-test-15011-facing-spawn.json`, log
`runtime/logs/facing-spawn.log`, traces `runtime/traces/facing-spawn`.
Both roster rows visible on Solaris and Mission1; Subvoltic Alpha, BetaPilot
Beta. Ready requests accepted at13:12:18/21. All Go tests pass. Captures are
local `runtime/facing-*.png`; no client files are committed.

## Targeted native shot checkpoint

Both clients entered combat at13:13. New mirrored spawns give clear mutual
sight, displayed 200.0m. On Subvoltic, BackSpace stabilized cursor mode; `t`
selected BetaPilot and displayed its target damage panel. `r` alone had not
selected it. LeftAlt .4s at13:14:06 produced the first verified native targeted
report: `weapon slot 0, target entity 1`, relayed by the server. Receiving
BetaPilot screenshot `runtime/facing-b-after-shot.png` shows the incoming
laser from Subvoltic. This establishes targeted firing and receiving effect,
not an authoritative damage result. Both clients remain running; do not launch
more. Targeting and fire captures are `runtime/facing-a-target-t.png` and
`runtime/facing-a-fired.png`.

## Targeted trial completion and damage receive checkpoint

BetaPilot also selected Subvoltic and fired slot0/target0 at13:15:10. The four
trial reports (two laser, two flamer) were all position/fire batches; new live
standalone delivery remains pending. Saved the two laser bodies in
`data/test-fixtures/combat-live-targeted-fire.json` with parser/peer conversion
regressions. No damage change established. Stopped only arrival-a/b at13:17:56,
before clock expiry; server396413 remains on15011. Both clients are now stopped.

Recovered independent AddDamage visitors0x69073270/0x690732d0 at table offsets
0x10/0x14. Their keys0x42/0x43, wire `c`/`d`, serialize five/four one-digit
payloads. First includes target entity; second applies to receiver's local
entity. They queue three AddDamage arguments and write a final byte to
Mech+0xdc. Fire's weapon-type branch (types outside10..16) only sets +0x2f4 to trigger ApplyDamage;
without queued records it cannot change armor. See combat-command-stream.md
for offsets and next steps. Added isolated `damage`/`local-damage` native
writer fixtures and exact-layout/envelope tests. No guessed live damage sent.

Full Go suite passes after spawn and captured targeted-shot coverage, including
the isolated damage-writer fixtures. Refresh generated site pages before push.
Next: recover hit location/amount, third AddDamage argument and piloting-target calculation, then integrate a controlled server-owned hit path. Keep at most two
clients; no VCR flags, no populated0x97 and no unrelated server restarts.

Final static follow-up: exported SetPilotingTarget0x69004000 identifies the
last damage-command byte (Mech+0xdc) as the piloting target. Correct calculation
is still needed. Damage's first argument indexes a combined component/armor
space (Damage0x69048143 compares against21); do not treat it as a simple HUD
armor index without recovering the mapping. Both saved pilots now have no
RoomID and Mission1 seats are empty. Server396413 alone remains on15011.

## Absolute damage semantics checkpoint

Resumed clean at b390ebc with server396413 alone, no clients. Native resolver
0x690500c0 subtracts incoming damage from current armor, queues slot21+armor
with the remaining value, and queues slot32+internal for overflow. Native
Damage0x690484c5 only lowers an armor value to the provided absolute value.
An isolated i386 harness (`scripts/native-damage-fixture.py`, saved, not yet
committed) runs the original resolver/GetDamageState with a bounded AddDamage
recorder and runs original Damage with a synthetic Reality getter. Results:
armor index4 15 minus5 -> [25,10,7]; armor3 minus5/internal20 -> [25,0,7] and
[36,18,7]; direct assignment10/repeat10/older12 leaves10 throughout. No Wine
clients, network or live injection used. Third value7 propagates as credit
entity through the damage event callback; exact presentation effects outside
non-destructive armor cases remain untested.

SendShotCommand0x69049f95..0x69049fd1 copies HitReturn entity+1 and part+1 into
fire object+0x31/+0x34. Local/direct resolution at0x6904a0dc passes HitReturn
entity to GetMech and part to armor resolver. Thus the field previously called
target is a collision report, not merely the HUD's selected target. No new
server-owned hit validation or damage emission is implemented yet.

## Collision-part implementation checkpoint

Full Go suite passes with the absolute damage encoder, native armor semantics
fixture, collision-part parsing and Mech region guard. Both captured laser
reports decode to region4. No damage command is emitted. Rebuilt and restarted
only15011 after checking its PID/command and absence of clients. Server PID
399951, binary `runtime/mpbt3025-collision-part`, cloned world
`runtime/world-test-15011-collision-part.json`, log
`runtime/logs/collision-part.log`, traces `runtime/traces/collision-part`.
No game clients were launched in this continuation. Previous live evidence
remains under facing-spawn traces/captures.

Completion checks: native armor fixture regenerated byte-for-byte identically;
native fire writer with hit-part10 round-trips its envelope. Both captured
laser frames also pass the server's new part guard. Full Go suite passes.
Related README/protocol/research/roadmap, script/data/test documentation and
site status are updated. Next recover stock weapon damage, piloting-target
calculation and hit validation before emitting absolute assignments. Armor
resolver synthetic internal20 is fixture input, not a recovered COM-4D stat.
Server399951 remains idle on15011; no clients, no unrelated servers touched.
