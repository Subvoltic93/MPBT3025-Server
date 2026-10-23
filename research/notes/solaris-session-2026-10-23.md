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

## One-shot armor diagnostic checkpoint

Resumed clean at8bd6ebc. Native CWeapon::Init0x6905f610 reads damage from
0x690b99b4+140*type and range bands from0x690b99c8+140*type. Type3 medium
laser damage5, bands45/90/135/180/225/270m. CMechObject constructor0x69041026
sets initial piloting target4. Added those pinned native table/constructor
values to the armor-resolution fixture.

Implementing opt-in `-experimental-combat-armor-probe`: one non-lethal stock
medium-laser armor reduction per target, only after valid active-room collision
references; client collision geometry remains trusted. No internal/critical/
destruction or repeated-hit piloting calculation. Uses initial piloting4 and
stock armor minus5, broadcasts one `c` assignment then `e` fire stream to both
clients, including shooter (local fire visitor skips its effect but applies
the queued target record). Default relay unchanged. Probe budget is cleared
on mission launch/departure/disconnect. Native live behavior pending. No
clients running yet; server399951 is still the old guard-only build on15011.

Armor diagnostic live trial started: server401100 (`runtime/mpbt3025-armor-probe`),
cloned world `runtime/world-test-15011-armor-probe.json`, log
`runtime/logs/armor-probe.log`, traces `runtime/traces/armor-probe`. Arrival-a
client401101, arrival-b401107; both nice10. At most two clients. Suite passed
before launch. Do not restart other listeners or start additional clients.

Both diagnostic clients reached Mission1, COM-4D and NOT READY rows populated
on Alpha/Subvoltic and Beta/BetaPilot. Shells0x4a0000f/0x4e0000f, WMframes
0x60dc32/0x60db85. Ready inputs sent. Capture `runtime/armor-a-mission.png`.
Next capture undamaged HUDs, one laser each direction, compare both displays.

## First synchronized live armor change

At13:46:51 Subvoltic slot0/target1/part4 triggered one diagnostic absolute
armor assignment15->10 (slot25, credit0, piloting4), followed by the same native
fire in one stream to both clients. BetaPilot own armor and Subvoltic target
panel both visibly changed center torso from gray to green. Evidence:
`runtime/armor-b-before.png`, `runtime/armor-b-after-first.png`,
`runtime/armor-a-target-before.png`, `runtime/armor-a-target-after.png`.
Both clients remain stable/running. This verifies synchronized diagnostic
armor presentation, not authoritative geometry or full combat damage rules.

## Synchronized armor trial completion

BetaPilot slot0/target0/part4 at13:47:32 produced the reverse probe15->10.
Subvoltic own torso and BetaPilot target panel both turned green, matching the
first direction. Captures `runtime/armor-a-after-reverse.png` and
`runtime/armor-b-target-after.png`. Subvoltic selected medium laser again
with `4` (wait for the HUD to refresh), then fired at13:48:48: same valid
slot0/target1/part4 report relayed with no third diagnostic assignment. Receiving
capture `runtime/armor-b-after-repeat.png` keeps the green torso. Exactly two
probe logs, one per entity. Both clients remained stable throughout.

Stopped only arrival-a/b after the repeat check, before timer expiry, to reduce
desktop load. Server401100 remains on15011 with the armor-probe trial world.
No VCR flags or populated0x97, at most two clients, no unrelated servers touched.
This proves the native queue-before-fire damage path and synchronized HUD state
for one non-lethal hit each direction. It does not prove independent collision
validation, repeated damage, internal/critical handling, destruction or results.

Saved `data/test-fixtures/combat-live-armor-probe.json`: exact client reports,
assignment/fire server bodies each observed in BOTH session traces, and the
repeat client shot. Regression replays now compare generated bytes against
those live receiving streams, including reverse direction and no repeat update.
Full Go suite passes after capture replay. Both saved pilot RoomIDs are cleared
by disconnect; verify no clients before the next trial. Server401100 remains
on15011 with probe enabled; ordinary startup defaults keep it disabled.

Final validation: all Go packages pass, native armor fixture regenerates exactly,
198 local site links resolve, diff whitespace check passes and final server
build succeeds (`runtime/mpbt3025-armor-probe-final`). Live server401100 uses
`runtime/mpbt3025-armor-probe`, same probe implementation. Both clients stopped;
saved rooms/seats cleared. Documentation is refreshed for the bounded live
success and remaining authoritative/full-damage/result gaps. Push this checkpoint
with the implementation and protocol-only capture fixture.

## Continuation checkpoint: position and weapon validation

Resumed from pushed commit `6a8164b` on a clean `main` checkout. Only the
isolated armor-probe server PID401100 is listening on `127.0.0.1:15011`; no
BT3025 clients are running and no other listeners are being changed. Current
work is offline native analysis of position serialization, range, and weapon
timing so the next live trial can test a constrained server-side combat path.

Native position wire `0x27` carries two signed four-digit coordinates first.
The signed reader subtracts `85^4/2`, matching `combatWriter.coordinate`;
captured live reports decode to `(0,10000)` and `(0,-10000)`, the launch
coordinates for the mirrored diagnostic spawns. `FillMech` multiplies setup
coordinates by `0.01`, so this representation agrees with the 200m separation.
The client includes position before each sampled shot, but the relay currently
does not retain it on the server. The six medium-laser values at weapon object
`+0x34..+0x48` are range bands `[45,90,135,180,225,270]`; their role as
accuracy bands versus a hard range limit has not been established. No range
gate or damage rule was added from that assumption. The live armor diagnostic
server and no-client state are unchanged.

Next inspect the weapon range/cadence consumers and decode the fire report's
collision coordinates. A server check should be based on those native meanings,
then tested with at most the two existing prefixes. Before enabling any new
live behavior, record another checkpoint.

## Collision point decoding checkpoint

Continued offline from `bb11595`; server401100 remains the only listener on
15011 and no clients are running. `SendShotCommand`0x69049e60 copies the three
HitReturn doubles from offsets+0x10/+0x18/+0x20, multiplies each by100, and
stores the resulting integers at fire-command offsets+0x4c/+0x50/+0x54. The
native fire writer emits signed four-digit values in bytes23..34. Both saved
live shots decode to `(-9936,13891,0)` and `(9930,13891,0)` hundredths.

`ParseClientCombatFire` now exposes these fields as `HitPoint` and flags whether
the report names a Mech. Regression decodes both original captured 0x81 frames;
the complete overlay Go suite passes. This establishes that they are copied
HitReturn coordinates, not that they prove a hit. Next compare them with
authenticated-room positions and recover axes/tolerance before authorizing
damage. No live behavior or process state changed; publish this checkpoint
before any new native trial.

Added `ParseClientCombatPositionCommand` for the first two signed coordinates
in native `0x27`. The armor-probe regression now decodes position and hit point
from each saved frame: shooter coordinates `(0,10000)` / `(0,-10000)` and
reported points `(-9936,13891,0)` / `(9930,13891,0)`, all before converting
hundredths to world units. These outputs document the distinct values but do
not imply a validated ray or known target center. Full overlay tests pass.

Further native trace: callback0x69073640 handles the local position-command
family only in combat state3. It scales position fields by0.01 and calls
Mech::SetPosition0x69045920; the client writer at0x6906f700 serializes those
fields into the 37-byte `0x27` command. This confirms the position update feeds
native pose state, but does not yet map the peer `0x41` application path or
recover the 3D collision volume. The server still has no retained authoritative
pose cache. Do not gate the hitpoint on these client reports until that receive
path and collision tolerance are understood.

## Continuation checkpoint: correcting the receive-path mapping

Resumed from pushed `13316c0`. The only live process remains diagnostic server
PID401100 on `127.0.0.1:15011`; no game clients are running. Inspected the
native callback table and command vtables from the installed original
`BTCombat.dll`. The pointer at `0x6908e6ac` confirms `0x69073640` occupies
visitor slot1. That slot index alone does not establish that the peer key
`0x41` command uses it. The function is combat-state3 and position-related,
but key `0x41` decoder field offsets and visitor callback have not yet been
connected with sufficient certainty. Older statements in the 2026-10-21
handoff identifying it as the peer visitor are corrected in that handoff and
in `combat-command-stream.md`.

Additional factory trace: the `0x41` registration thunk at `0x6906f450`
registers through `0x69073e40`; its factory constructs a 0x50-byte wrapper
through `0x6906f1c0`, which initializes command storage then invokes decoder
`0x6906f3d0`. The wrapper has outer and embedded vtable pointers, so treating
the registration's `0x6908e284` value as the decoded command object's sole
vtable was also unsafe. The next trace must follow the wrapper's generic
receive/visitor call and identify which subobject it passes.

No code or live behavior changed. Next trace the generic receive/visitor
dispatch from the key `0x41` factory instance to the callback, including
object field offsets. Until confirmed, do not add an authoritative hit gate
based on reported hit points. No restart, new client, VCR flag, or `0x97`
response was used.

## Receive visitor trace resolved

Follow-up disassembly connected the `0x41` wrapper to its visitor. Factory
`0x6906f1c0` passes the embedded subobject (`wrapper + 4`) to decoder
`0x6906f3d0`; this places the entity at wrapper `+0x30` and pose fields at
`+0x34` onward. Those offsets match callback `0x69073640`, which reads that
entity, scales the pose, and calls `Mech::SetPosition` in combat state3.
Thus `0x69073640` is the native server peer-position visitor. The earlier
unresolved-mapping checkpoint above is superseded by this field-offset match.

Next recover axis transforms and collision volume/tolerance before applying
any hit gate. Client-reported HitReturn coordinates remain untrusted, and the
server does not yet retain the relayed pose. No code or process state changed.

## Reported-position cache checkpoint

Added a per-mission cache of the most recent reported X/Y plus 15 opaque native
pose-field bytes, keyed by the authenticated compact combat entity. The relay
updates it only for an active mission member after the full native frame
passes validation. New mission launch, mission departure, and disconnect
clear that mission's cache. The cache is observational and does not influence
relay or damage decisions.
Regression now checks captured coordinates, authenticated entity selection,
corrupt-frame rejection, and room isolation. Full overlay Go suite passes;
the two focused tests also pass under the Go race detector. The new server
build succeeds. Existing native server PID401100 and its no-client state
remain unchanged. Axis and collision interpretation remain open.
