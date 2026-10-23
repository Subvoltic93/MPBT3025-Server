# Research Notes

This file summarizes the current emulator’s evidence and design decisions. Dated experiments, trace references, and implementation history are catalogued in the [server emulator research notes](research/notes/server-emulator-research.md).

## Scope and method

The project reconstructs the server-facing behavior needed by a local MPBT 3025 client. Packet layouts are implemented in Go from observed traffic and controlled client behavior. A protocol detail is treated as confirmed only when its wire layout and client response have both been observed; a decoded field name alone does not establish its gameplay meaning.

The project layout borrows the broad separation used by [KenzHuman’s `mpbt-server`](https://github.com/kenhuman/mpbt-server): source, research, utilities, tests, and data have distinct homes. The language, package boundaries, code, and documentation here are independently maintained.

## What is represented today

- Aries transport framing is a 12-byte little-endian header followed by a bounded payload. Game messages inside data frames use a six-byte, big-endian PDU header.
- The connection path handles the observed local startup exchange, configuration synchronization, pilot creation request, room and planet joins, room exit, settings responses, chat, ready/mech selection, combat initialization, and combat command relay.
- Campaign and session state are modeled separately from packet codecs. The current store persists the world as JSON and clears transient room presence on startup. `src/db/` is only a placeholder; no database driver or schema is active.
- The default world generator creates 121 generically named planets, Solaris city/mission staging, and prototype campaign rules. Operators can load external seed tables through `MPBT3025_SEED_DIR`; those tables are not part of this source tree. Many non-Solaris values such as adjacency, rank progression, and economy remain approximations.
- Live session membership is separate from persisted pilot records, so a disconnected pilot should not remain in a visible roster. Solaris staging membership is restricted to the same city, including that city's seated pilots. Mission counts are calculated from occupied or AI seats; the Solaris list maps defender seats to Alpha and attacker seats to Beta in the current implementation.
- Historical two-client trials reached Solaris staging and native combat. Position and fire commands were relayed. The October 23 trial reconfirmed the same-city counts, team labels, Back/rejoin, chat filtering, and ready/cancel behavior after the repository move.

## Confidence boundaries

**Observed or directly exercised:** basic Aries connection bootstrap; Solaris entry and staging; two-client occupant updates; mission join and seat changes; countdown/launch sequence; native cockpit initialization; and room-scoped position/fire relay.

**Implemented but not fully validated end to end:** character creation persistence semantics, House chat, full campaign navigation, economy screens, authoritative room-count behavior across reconnect and multi-room cases, and stable reconnect behavior.

**Unknown or incomplete:** the meaning of every field in planet-state records; the client’s complete split-buffer reassembly behavior; full Team chat semantics; authoritative movement, hit detection, damage, destruction, match end, score, rank, and rewards.

## Room roster and side labels

The server has two related sources of room information. Connected planet occupants are kept in the session hub; mission occupancy and seat sides come from the persisted world store while a connection is active. The intended rule is that lobby player counts come from live sessions, and mission player counts come from occupied seats. Solaris wire fields reverse the human-facing Alpha/Beta label order used for defender and attacker seats.

The October 23 inspection found a remaining source-level error: city-specific staging RoomIDs used a planet-wide peer selection. That could include occupants from another Solaris city in the roster/count and send arrival/departure events across city boundaries. A failing regression reproduced the leak, and a shared presence-area check now restricts Solaris membership to planet plus city. The test also checks the complete roster snapshot and disconnect isolation.

The native trial used two pilots in city 1 and confirmed one-pilot Alpha counts/preview, empty counts after Back, both pilots in the settled staging roster, and two correctly labeled mission seats. All reached the peer; Team and House stayed with the sender for opposite sides and unassigned Houses. Both Ready states and cancellation rendered correctly; a second countdown released both clients into active native combat cockpits. This does not demonstrate native cross-city navigation or same-side/assigned-House delivery. See the [October 23 checkpoint](research/notes/solaris-session-2026-10-23.md) for exact trial paths and limitations.

## Evidence and local material

Implementation details and historical traces are indexed in the dated notes. The [game manual](reference/manual/Multiplayer-Battletech-3025_Manual_Win_EN.pdf) and [SDS](reference/sds/btech3025-sds-v1.1a.doc) are stored under `reference/` and listed on the project files page. Packet summaries under `runtime/traces/` and campaign snapshots under `runtime/` are local development output, ignored by version control. Utilities that inspect supplied client files require `MPBT3025_CLIENT_ROOT`; the original client files and third-party reference checkout remain elsewhere.

When investigating a regression, record the exact server flags, a clean world snapshot, client-visible result, and the corresponding trace. Keep decoded observations separate from interpretation, and promote an experimental flag only after a repeatable client trial.

## Fire selection and standalone-event follow-up

Native fire writers and receiver visitor establish weapon slot and target-plus-one references. Targeted synthetic fixtures execute the original serializers; the server checks slots against its stock COM-4D initialization and targets against the launched room roster. This prevents invalid native object references but does not validate geometry or damage.

The October 23 live trial found a 36-byte standalone fire command that the old position-first relay dropped. A later combined position/fire event passed the new reference checks. The parser now accepts standalone fire and complete batches, rejects unknown/truncated tails, and retains the position-only preparation requirement. The exact standalone capture has active-room relay regression coverage. Standalone delivery after this correction is capture-replayed in tests; a new native-client standalone relay trial remains pending. That earlier trial did not establish a target lock or receiving-client damage; the controlled-spawn follow-up below establishes target selection and incoming effects.

## Experimental spawn arrangement

Native `FillMech` scales setup coordinates by 0.01 and computes facing toward the map origin (`0x6904207e`–`0x690420fd`). The experimental launcher now mirrors opposing lances across that origin: first seats are 200 metres apart, with 30 metre seat spacing. This controlled diagnostic arrangement replaces the almost parallel, 20 metre separation used previously. The October 23 follow-up confirmed mutual sight, 200 metre target selection, targeted laser reports in both directions and the incoming laser effect. No damage change was established. These are diagnostic spawns, not recovered production spawn rules.

## Absolute armor assignment evidence

`native-damage-fixture.py` executes the original armor resolver, damage-state lookup and non-destructive armor assignment on synthetic arrays. It replaces AddDamage with a bounded recorder and the Reality getter with an inert object; no Wine clients or networking run. Native results establish absolute remaining armor values, armor-to-internal overflow, and idempotent armor assignment. `SendShotCommand` also identifies the fire report’s entity and part as collision results. The relay validates Mech parts 0–10, and a native-compatible absolute damage encoder is available. Damage emission and authoritative hit checks remain pending. Detailed offsets and limitations are in the [combat notes](research/notes/combat-command-stream.md).
