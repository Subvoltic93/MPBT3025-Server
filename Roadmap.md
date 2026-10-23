# Roadmap

The goal is a usable local multiplayer server with a clear boundary between confirmed behavior and prototype rules. Progress below reflects implementation and live-session evidence through October 23. Same-city counts and side labels were reconfirmed; city isolation is regression-tested and still needs a native trial across two cities.

| Stage | Status | Outcome |
| --- | --- | --- |
| Aries listener and game PDU framing | Done | Local TCP bootstrap and packet codecs are separated into protocol packages. |
| Pilot and campaign state | Partial | JSON persistence, pilot records, Solaris locations, rooms, seats, and prototype campaign rules exist. Creation and progression need full client-screen verification. |
| Solaris staging and occupants | Partial | Two-client counts, Alpha/Beta labels, Back/rejoin and settled rosters were reconfirmed. Cross-city isolation is fixed and regression-tested; native cross-city/reconnect cases remain. |
| Mission readiness and launch | Demonstrated | Seat selection, readiness/cancellation, countdown, preparation barrier, and native combat entry were reconfirmed with two clients on October 23. |
| Combat command relay | Partial | Room-scoped position/fire relay is demonstrated. Standalone/batched fire parsing and stock weapon/room target checks have native-fixture and captured-packet coverage. Geometry, ammunition and damage remain unauthoritative. |
| Combat resolution and results | Partial | One non-lethal diagnostic armor update per target synchronizes both clients. Authoritative hits, repeated damage, internal/critical damage, destruction and results remain unfinished. |
| Campaign services and economy | Later | Connect battle results to rank, credits, ownership, salary, and client-facing screens. |
| Persistence adapter | Later | Choose a storage engine and migration approach only when JSON persistence no longer meets the server needs; keep storage behind a narrow adapter boundary. The `src/db/` files are placeholders today. |
| Operational hardening | Later | Add restart/reconnect policy, configuration guidance, clean logging, and repeatable end-to-end scenarios. |

## Next work

1. Exercise native cross-city navigation and reconnects against the city-scoped roster correction; keep same-side seat assignment as a separate unverified case.
2. Preserve the repeatable same-city scenario: one pilot staged, peer joins/leaves, both join, chat channels, ready/cancel, then native combat entry.
3. Reconcile the confirmed key `0x41` peer-position visitor's pose fields with world-axis transforms, then recover collision volume/tolerance and whether `HitReturn` coordinates can be validated against server-held peer poses. The server caches reported X/Y plus 15 opaque native pose bytes per authenticated entity for diagnostics and the opt-in cumulative damage policy; collision reports remain untrusted.
4. Extend the verified diagnostic assignment/fire path into server hit validation and remaining-armor tracking only after pose and hit geometry are understood. Preserve separate component/internal/destruction rules and recover the piloting-target calculation. Clear mutual sight, target selection and targeted laser relay in both directions are now demonstrated. New native standalone-fire delivery remains pending; the follow-up shots were position/fire batches.
5. Implement match completion only after hit/damage behavior and the client's result-message layout are supported by evidence.

## Completion criteria

The multiplayer loop is complete when two clients can enter a mission, receive correct roster and team displays, move and fire, see consistent hit and damage outcomes, and return to a stable result screen with persisted rewards. Each step must be repeatable from a clean local world snapshot and trace.

## Experimental spawn arrangement

Native `FillMech` scales setup coordinates by 0.01 and computes facing toward the map origin (`0x6904207e`–`0x690420fd`). The experimental launcher now mirrors opposing lances across that origin: first seats are 200 metres apart, with 30 metre seat spacing. This controlled diagnostic arrangement replaces the almost parallel, 20 metre separation used previously. The October 23 follow-up confirmed mutual sight, 200 metre target selection, targeted laser reports in both directions and the incoming laser effect. The later opt-in armor diagnostic verified one synchronized non-lethal armor change per target. These remain diagnostic spawns, not recovered production spawn rules.

The opt-in armor diagnostic has now verified native damage delivery and both clients’ own/target HUD synchronization in both directions. Preserve the exact receiving-stream fixtures while implementing full damage rules; a reported collision alone is not authoritative hit validation.

## Cumulative damage checkpoint

The opt-in cumulative COM-4D laser mode now has server-owned armor/internal totals, native-fixture-tested transfer and weapon-disable records, recharge, range and reported-pose plausibility checks. Native world axes are recovered. Next verify repeated damage and destruction on both clients, then recover mesh collision/occlusion and piloting-target evolution. Full authority, heat, criticals, ammunition weapons and match results remain open.
