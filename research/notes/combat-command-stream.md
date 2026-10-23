# Native combat command stream (0x81)

Historical local paths in this document are relative to the original workspace root. Original client assets, client-derived research, and visual evidence remain in the separate workspace-root `client/` directory and are not included in this server project.

Static and executable-fixture evidence from the supplied September 2001
BTCombat.dll; preferred image base 0x69000000. The server now encodes the
command envelope and a controlled stock COM-4D setup. Two-client native combat
entry, position relay and fire relay have been demonstrated. Standalone fire
parsing and reference validation now have captured-packet/native-fixture tests;
authoritative hit/damage and match completion remain unfinished.

`ReceiveCombatData` (0x6905cd60) forwards the PDU body and length to
0x690755c0. The bounded stream is processed by 0x69075060 before dispatch.
The command type reader (0x69074b30) subtracts 0x21 from one wire byte and
looks up the result in the factory registry at 0x690d4b50. Parsed commands
are queued through 0x690753b0. Command application uses the visitor table
at 0x6908e6a8.

| Command | Registry key | Encoded type byte | Registration | Visitor offset | Apply callback |
| --- | --- | --- | --- | --- | --- |
| NewMechCommand | 0x40 | 0x61 (`a`) | 0x6906f040 | 0x34 | 0x69073610 |
| Ready for combat | 0x3e | 0x5f (`_`) | 0x69072440 | 0x44 | 0x69073a00 |
| TitleCommand | 0x48 | 0x69 (`i`) | 0x69072d40 | 0x48 | 0x69073a10 |

The command-to-visitor trampolines are 0x6906f1a0, 0x69072540, and
0x69072e40 respectively. The callbacks provide independent semantic
confirmation: NewMechCommand calls exported `FillMech` at 0x690411a0;
TitleCommand calls `FillReality` at 0x69057a60, which ends by calling
`PrepareForCombat` at 0x69056160. The ready callback calls exported
`ReadyForCombat` at 0x690570e0, including `GraphicsManager::EnterCombat`.
Thus the stream has explicit preparation and entry commands beyond the
shell-level 0x80 message. The later executable fixtures and live preparation
barrier trial below established the required initialization sequence.

## Encoding details recovered so far

- Small values and command types use one character with bias 0x21.
- Multi-character integers use base85, also biased by 0x21. Reader
  0x6906a180 multiplies by 0x55 for each following digit; its width argument
  1..4 selects 2..5 encoded characters. Do not serialize these as raw u32.
- The two-/three-width signed helpers (0x69074c90/0x69074cc0) subtract
  fixed biases after decoding. Other helpers decode bounded unsigned values.
- Preprocessing removes outer framing (0x690751c0), checks/removes a
  three-character checksum (0x69075110), then removes an inner prefix
  (0x690750b0). The matching serializer inserts 0x21 as the inner prefix
  and ESC framing around the checksummed stream. Executable native fixtures
  now confirm both checksum directions and terminal NUL semantics (below).
- TitleCommand decoder is 0x69072c90. It reads a string, two bytes, four
  bounded integers, then several one-character values. These member
  offsets are relative to its serialization subobject; account for the
  subobject offset before matching `FillReality` accesses.

The ready factory decoder at 0x69072470 constructs through 0x690722d0,
which invokes the no-op decoder 0x6906b7f0. Its payload therefore appears
empty after the type byte; the native writer fixture confirms exactly `_`.

Native fixtures now cover Title, NewMech, Ready, and stock COM-4D equipment.
A controlled setup trial is behind `-experimental-combat-init`; it is disabled
by default. It supports peer position/fire relay, with no authoritative hit/damage simulation.

## Stream checksum and title layout follow-up

The checksum routine at 0x69075200 is the same 19-bit LFSR described in
`../../../../MPBT3025/references/solaris-protocol-reference/solaris_server.py` (which documents the
related Solaris COMBAT stream). Its seed selection is directly visible in the
BTCombat DLL: when the stream context flag at `this+4` is clear, the seed is
0xC4545 (804165); when set, the 32-bit addition wraps to 0xA5C45 (678981).
The reference identifies 678981 as the server-to-client COMBAT checksum seed.
Each checksum is encoded as three base-85 digits after the inner-prefix byte
and command payload. The title command decoder reads a length-prefixed string,
two one-byte fields, four two-byte-width base-85 integers, one one-byte field,
three more one-byte fields, and a final one-byte field. Its serialization
subobject fields sit at offsets 0x2c and 0x3c..0x54. `FillReality` consumes
these as map/reality setup values, and then calls `PrepareForCombat`.

This is enough to construct a byte-level envelope, but the semantic values for
the TitleCommand and the complete NewMechCommand remain unverified. Do not
send a guessed 0x81 stream yet: a checksum-valid packet can still install an
invalid terrain or mech state and crash the native client. Next match the
TitleCommand fields against the known Solaris CombatInfo and recover the
NewMechCommand decoder/model fields before the first controlled stream trial.

## Executable native fixtures and first setup trial

`scripts/native-combat-fixture.py` maps the supplied DLL's PE sections
into an isolated Linux i386 executable at their original addresses. It calls
the original command writers (Title0x69072be0, NewMech0x6906e830,
Ready0x6906c600), prefix/checksum/framing routines, and receive preprocessing.
It substitutes only a bounded raw-buffer write method and memmove; it does not
initialize a DLL, connect to a server, attach a debugger, or modify a client.
The DLL SHA256 is pinned to prevent applying offsets to another build.
Generated JSON fixtures in `data/test-fixtures/combat-native-*`
record original bytes and both direction checksum results. Native preprocessing
must recover the exact original command before a fixture is accepted.

Example: the native ready command is5f, server frame1b215f41252f1b. The wire
payload is u32 big-endian length followed by that frame **and its NUL**:
000000081b215f41252f1b00. EXE writer0x520130 and handler0x4d8610 show that the
counted length is forwarded unchanged to ReceiveCombatData. The receive
preprocessor removes three bytes from the framing, accounting for the NUL.
Server checksum seed678981 and client seed804165 now have native fixtures.
`CombatData` matches all native frames; Title/stock-COM-4D command encoders
match original command writers. All Go packages passed after integration.

Recovered whole-object Title fields: +0x40 local combat entity ID; +0x41
climate (0 Desert,1 Temperate,2 Tropic,3 Arctic); +0x44 terrain number;
+0x48 gravity in hundredths, converted to negative acceleration; +0x4c ground
friction percent; +0x50 air pressure; +0x54 heat penalty. Exported getter
functions independently establish the physics meanings. FakeStart0x6905c350
sets gravity980, friction100, pressure100, heat0. The +0x55 field is the EXACT Mech count. FakeStart sends its last Mech
through a shared tail call; an earlier player-count+1 interpretation was wrong.
The setup uses +0x55=len(players), +0x56=2, +0x57=0, +0x58=5.
AddNewMech compares created count (+0xa4) with declared count (+0x108);
only equality advances Reality state (+0x140) from 1 to 2.

FakeACat0x6905aff0 has an explicit nativeMechID8 branch0x6905b696 for COM-4D /
Commando. It supplies the stock characteristics, eleven armor records, three
weapon records, one ammunition record, and the thirty shared component
records at0x6905c096. These are now encoded and checked against the original
NewMech writer, including nonempty lists. VC6 vector objects have allocator
padding then begin/end/capacity pointers; account for the extra four bytes.
A native writer quirk at0x6906e8be serializes the command type0x40 into the
first numeric field instead of whole-object+0x90. The decoder stores it there,
but FillMech does not read that field; the encoder follows the native fixture.

Current controlled trial uses only starter COM-4Ds, compact combat IDs0..N-1,
combat teams0/1 and recipient-specific local ID. Title and Mechs are sent
first; Ready MUST be deferred until PreCombatInit initializes the HUD. Sending
Ready immediately caused a native K2D.dll NULL+0x178 crash through
ReadyForCombat / CHUDBogie::AddBogie. Correct count plus deferred Ready
rendered BOTH complete desert cockpits and weapons HUDs in the stage3 trial.
Screenshots: client/.runtime/captures/native3-after-setup-{a,b}.png.

The opt-in server now waits for a checksum-validated 37-byte native position
command (wire 0x27) from EVERY launched pilot before broadcasting Ready once.
These commands begin after PreCombatInit reaches state3. The barrier has a
regression covering early release, other-room input, corrupt frames and duplicate
release. Live barrier verification PASSED: both setup packets at13:55:54, both
prepared and Ready broadcast at13:55:58. Captures
client/.runtime/captures/barrier-released-{a,b}.png show Stand By gone and
active match clocks on both clients, with no native crash. The shell continues using
one-based seats and teams1/2. At this checkpoint movement relay was still pending (see the later trials below);
cockpit rendering alone does not establish a complete multiplayer battle loop.


Native position relay: client command wire0x27 (key6, writer0x6906f700)
contains signedX4, signedY4, unsigned3, seven signed3 and unsigned4 fields.
Server position wire0x62 (key0x41, writer0x6906f350) contains entityID1,
then signedX4, signedY4, unsigned3 and the first four signed3 fields.
scripts/native-combat-fixture.py now has client-position and position
kinds proving the exact shared wire bytes. CombatPeerPosition validates the
client command and inserts the sender entity from the launched room roster.
Live relay trial at14:05:11 passed: both clients stayed in combat and peer
indicator changed when Subvoltic moved forward at20kph. See the Solaris session
checkpoint for process IDs, logs, traces and captures. This position trial did not yet exercise fire; see the following weapon-event trial.


Weapon event: a native Left Alt shot is batched after position in the same
client0x81 frame. The second command is36-byte wire0x2b (client key0x0a,
writer0x69070c20). Server key0x44, writer0x69070810, serializes36-byte
wire0x65: key, authenticated shooterID, client fire bytes[1:35]. The client's
last fire byte is omitted. Native executable fixtures for fire and server-fire
confirm exact bytes. The server now relays this command to peers in the active
room after the position packet. In the14:13:05 live trial, a shot generated
the combined client frame and a server fire packet to the peer without a
native crash. The peer effect and damage were not visually confirmed.
Receiver visitor0x69073770 includes a local-target branch; further work must
validate targets and handle damage/results authoritatively.


## October 23: fire references and standalone batches

Client writer `0x69070c20` writes type `+`, weapon slot (whole object `+0x30`),
then target-plus-one (`+0x31`). Server writer `0x69070810` inserts shooter at
`+0x30`, placing weapon at `+0x31` and target at `+0x32`. Receiver visitor
`0x69073770` indexes the shooter's weapon and subtracts one from the target
before a Mech lookup. Zero is untargeted. The target remains a client report;
its presence alone cannot prove a hit or damage.

Synthetic overrides in `native-combat-fixture.py` now generate targeted client
and server fixtures for slot 2, target entity 1. They execute the original
writers and round-trip through native envelope preprocessing. The Go mapping
matches those bytes. The server restricts stock COM-4D slots to 0..2 and
nonzero targets to entities in the launched room; it still trusts reported
geometry, weapon timing and ammunition.

A live shot at 12:33:10 arrived as a standalone 36-byte `+` command, slot 0,
target -1. The old parser required position first and silently dropped it.
The 12:34:13 combined position/fire packet was accepted by the reference
checks (slot 2, untargeted) and relayed. The parser now accepts standalone fire
or an optional leading 37-byte position followed by complete 36-byte fires.
Unknown/truncated tails reject the entire batch. Standalone fire never counts
as a preparation acknowledgement. The captured standalone packet exercises
the real envelope parser and room-scoped relay in regression tests; multiple
fires are tested as complete batches, not claimed as a new live observation.

No opponent lock, targeted shot, projectile impact or receiving-client damage
was established in this trial. Camera motion during focus complicated aiming.
Next establish clear mutual sight and correlate a reported target with peer
state; do not apply guessed damage based only on a target field.

### Experimental spawn arrangement

Native `FillMech` scales setup coordinates by 0.01 and computes facing toward the map origin (`0x6904207e`–`0x690420fd`). The experimental launcher now mirrors opposing lances across that origin: first seats are 200 metres apart, with 30 metre seat spacing. This controlled diagnostic arrangement replaces the almost parallel, 20 metre separation used previously. The October 23 follow-up confirmed mutual sight, 200 metre target selection, targeted laser reports in both directions and the incoming laser effect. No damage change was established. These are diagnostic spawns, not recovered production spawn rules.

## Targeted laser trial and separate damage callbacks

The mirrored-spawn follow-up selected BetaPilot at 200m using `t`, then
LeftAlt. Server log at13:14:06 confirms Subvoltic slot0/target1; BetaPilot
slot0/target0 followed at13:15:10. The receiving screenshot shows the incoming
laser; no armor/damage change was established. Both captured position/fire
batches are in `data/test-fixtures/combat-live-targeted-fire.json` and replayed
by tests. All four trial shots were batched; new native standalone delivery
remains pending despite the existing captured standalone regression.

The fire visitor 0x69073770 does not call AddDamage. Its local-target branch
queues a presentation object; its final weapon-type branch (types outside10..16) sets Mech+0x2f4,
which the update loop at0x69049e2e uses to call ApplyDamage. AddDamage must
already have queued damage records for that call to change armor.

Two separate receive commands provide that queue:

| Key / wire | Native writer | Visitor / table offset | Payload (whole-object fields) |
| --- | --- | --- | --- |
| 0x42 / `c` | 0x6906baa0 | 0x69073270 / 0x10 | +0x30 entity, +0x31/+0x32/+0x33 AddDamage arguments, +0x34 state |
| 0x43 / `d` | 0x6906be00 | 0x690732d0 / 0x14 | +0x30/+0x31/+0x32 AddDamage arguments, +0x33 state; receiver uses local entity |

All payload fields use one base85 digit. Registration0x6906bb60 sets key0x42;
0x6906be60 sets0x43. Both visitors require Reality state3. They call
AddDamage0x6904a450, then assign their final byte to Mech+0xdc. The third
AddDamage argument is widened to ushort; its meaning needs further confirmation.
The final byte is the piloting target: exported SetPilotingTarget0x69004000
writes precisely Mech+0xdc. Its correct server calculation remains unrecovered. ApplyDamage0x6904a5d0 later walks the
queued triples and calls Damage0x69048100. Synthetic native fixtures now
confirm the exact serializer/envelope layouts. These commands are not yet
emitted: target selection alone does not determine location, amount or the
remaining state fields. Next recover those values and server hit rules before
attempting a controlled damage trial.

## Absolute armor assignments and collision parts

The native damage command's second AddDamage argument is an **absolute remaining
value**, not an incoming damage delta. Resolver0x690500c0 subtracts incoming
weapon damage from Mech armor at+0x190, queues slot21+armor with the remainder,
and transfers excess to internal structure at+0x1e8 (slot32+internal). Original
Damage0x690484c5..0x690484e1 only replaces an armor value if the new value is
smaller. Applying a delta directly would corrupt state.

`scripts/native-damage-fixture.py` executes that resolver and original
GetDamageState0x69047fd0 with synthetic current/max armor arrays and internal
structure. AddDamage is replaced with a bounded triple recorder. It separately
executes original Damage with an inert Reality getter and non-local Mech to
avoid rendering/server callbacks. These substitutions are explicit in the
fixture; no native server initialization, clients or networking run.

| Synthetic input (armor region 4, credit entity 7) | Native queued records: slot / remaining / credit |
| --- | --- |
| Armor 15, incoming 5, internal 20 | 25 / 10 / 7 |
| Armor 3, incoming 5, internal 20 | 25 / 0 / 7; 36 / 18 / 7 |
| Direct armor assignment 10, repeat 10, older 12 | Observed armor 10, 10, 10 |

This fixture does not cover destruction, critical hits, depletion of internal
structure, transfer chains, missile impacts or piloting-target calculation.
The third argument propagates unchanged into damage event callbacks as a
credit entity. Its server use still needs authenticated attribution.
`CombatDamageCommand` encodes the explicit-entity `c` command and rejects field
overflow. It is deliberately not yet emitted by the relay.

SendShotCommand0x69049f95..0x69049fd1 copies HitReturn's entity+1 and part+1 to
client fire object+0x31/+0x34. The latter serializes as three unsigned base85
digits at command bytes3..5. Direct/local resolution0x6904a0dc passes those
HitReturn values to GetMech and the armor resolver. Therefore the wire entity
is a collision report, rather than merely the selected HUD target. Terrain
collisions use another part space when no Mech entity is reported. Both saved
opposite-direction laser batches decode to Mech armor part4. The relay now
rejects Mech reports with missing/out-of-range parts (valid0..10), while
preserving the separate terrain/untargeted space. This validates references;
it does not independently validate collision geometry or award damage.

## Live queue-before-fire armor diagnostic

`CWeapon::Init`0x6905f610 reads type-dependent damage at0x690b99b4+140*type
and range bands at0x690b99c8+140*type. Type3 medium laser has damage5 and
bands45/90/135/180/225/270m. Mech constructor0x69041026 sets piloting target4.
These pinned static inputs are recorded by the native armor fixture tool.

The opt-in `-experimental-combat-armor-probe` allows exactly one non-lethal
medium-laser armor assignment per target in an active native mission. It uses
stock COM-4D armor minus5 and initial piloting4, and trusts reported collision
geometry. It never handles depletion, internal/critical damage, destruction,
or further damage to that target. The allowance resets on launch/departure/
disconnect. This is a bounded native receive/routing diagnostic.

For that hit it constructs one stream: explicit-entity `c` assignment followed
by `e` fire, delivered to both shooter and target clients. Visitor0x690737be
skips effects for the local shooter but continues at0x69073967 to request
application of the queued target records. Sending only to peers would leave
the shooter's local target display unchanged.

Native trial13:46:51 Subvoltic->BetaPilot and13:47:32 BetaPilot->Subvoltic
both reported slot0/part4. Each assignment changed center torso15->10, and
both own and target HUD panels visibly changed gray to green. Both session
traces contain the same stream for each direction. Subvoltic's later slot0/
part4 shot at13:48:48 relayed without a third diagnostic assignment. Clients
stayed stable. Exact client reports and server bodies are preserved in
`data/test-fixtures/combat-live-armor-probe.json` and replayed through the
server tests. Local screenshots remain in runtime, not the public repository.

Next implement independent collision checks, server armor/internal state,
weapon cadence and correct piloting-target updates. Preserve this verified
assignment-before-fire order while extending damage; the diagnostic does not
establish authoritative hits or match completion.

## Position and range follow-up

Native client position command `0x27` begins with signed four-digit X and Y.
The decoder subtracts `85^4/2`, the inverse of the setup encoder. Captured
live position reports decode to `(0,10000)` and `(0,-10000)`, matching the
mirrored diagnostic launch coordinates; `FillMech` scales those coordinates
by `0.01`, producing the verified 200m pair spacing. The client sends position
alongside the sampled fire report, but the server does not retain positions.

`CWeapon::Init` copies six medium-laser range values to weapon object
`+0x34..+0x48`: `[45,90,135,180,225,270]`. The use of those values as accuracy
bands or as a hard firing range has not yet been confirmed, so they do not yet
support a server range gate. The fire visitor consumes additional reported
coordinates at object `+0x4c`, `+0x50`, and `+0x54`. `SendShotCommand`
0x69049e60 copies `HitReturn`'s three doubles from offsets `+0x10`, `+0x18`,
and `+0x20`, multiplying each by100 before storing them. The fire writer
serializes these as signed four-digit values at client command bytes23..34.
Both saved live shots decode to `(-9936,13891,0)` and `(9930,13891,0)` in
hundredths of a world unit. `ParseClientCombatFire` now exposes that reported
point, and a regression decodes both original captured frames. This confirms
the point's source and scale; it is still client supplied. Compare it against
authenticated-room positions and recover the coordinate axes and native hit
tolerance before using it to authorize damage.
