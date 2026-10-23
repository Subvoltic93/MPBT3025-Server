# Roadmap

The goal is a usable local multiplayer server with a clear boundary between confirmed behavior and prototype rules. Progress below reflects the implementation and existing live-session notes; room counts and side labels remain open because a mismatch has been reported.

| Stage | Status | Outcome |
| --- | --- | --- |
| Aries listener and game PDU framing | Done | Local TCP bootstrap and packet codecs are separated into protocol packages. |
| Pilot and campaign state | Partial | JSON persistence, pilot records, Solaris locations, rooms, seats, and prototype campaign rules exist. Creation and progression need full client-screen verification. |
| Solaris staging and occupants | Partial | Two-client presence was demonstrated. Recheck room counts, stale occupants, reconnects, and Alpha/Beta labels across join/leave cases. |
| Mission readiness and launch | Demonstrated | Seat selection, readiness, countdown, preparation barrier, and native combat entry were exercised with two clients. |
| Combat command relay | Partial | Room-scoped position and fire relay works in recorded trials. The server does not yet own authoritative simulation. |
| Combat resolution and results | Next | Validate targeted hit messages, apply damage consistently, detect destruction/end state, and return native results. |
| Campaign services and economy | Later | Connect battle results to rank, credits, ownership, salary, and client-facing screens. |
| Persistence adapter | Later | Choose a storage engine and migration approach only when JSON persistence no longer meets the server needs; keep storage behind a narrow adapter boundary. The `src/db/` files are placeholders today. |
| Operational hardening | Later | Add restart/reconnect policy, configuration guidance, clean logging, and repeatable end-to-end scenarios. |

## Next work

1. Reproduce the reported Solaris count/team-label mismatch with two clients and inspect the same session’s packet trace.
2. Compare roster updates, room occupancy fields, and seat counts after each join, leave, disconnect, reconnect, and side assignment. Correct the source of truth and the Solaris defender/attacker field mapping together.
3. Re-run the two-client mission launch and combat relay after the roster correction.
4. Trace a controlled shot through the client/server command stream, determine whether the client reports a hit or only a fire event, and identify the smallest server-owned damage step.
5. Implement match completion only after hit/damage behavior and the client’s result-message layout are supported by evidence.

## Completion criteria

The multiplayer loop is complete when two clients can enter a mission, receive correct roster and team displays, move and fire, see consistent hit and damage outcomes, and return to a stable result screen with persisted rewards. Each step must be repeatable from a clean local world snapshot and trace.
