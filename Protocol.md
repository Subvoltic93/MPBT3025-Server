# Protocol and Server Operation

This document describes the current Go server implementation. It covers both network addresses and message addresses (PDU IDs). A numeric ID listed below is handled only in the direction and context stated.

## Listener addresses

The server speaks TCP with Aries framing. It does not serve HTTP.

| Address | Use |
| --- | --- |
| `127.0.0.1:15000` | Default local bootstrap listener (`-listen`). It accepts the client’s primary connection and handles lobby, planet, room, and combat traffic. |
| `127.0.0.1:15001` | Optional secondary Aries listener (`-world-listen`). It is started only with `-planet-redirect`; a successful planet join can be redirected there as an experiment. |
| `127.0.0.1:15011` | Development trial address used for the current isolated test instance. It is selected explicitly with `-listen 127.0.0.1:15011`; it is not a second default service. |

All default listeners bind to loopback, so they accept connections from this machine. Use `-listen` or `-world-listen` to change a bind address. Binding to a non-loopback interface makes the service reachable on that interface.

## Direct client launch argument

The tested local launch starts the existing game through `client/scripts/launch.sh` and passes `aol:<server-host>:<server-port>:key:<local-key>` to the executable. For the default listener, the argument is `aol:127.0.0.1:15000:key:local-dev`. `aol:` is the legacy startup argument that selects the Aries TCP endpoint; the host and port must match the server's `-listen` address. For a LAN server, bind with `-listen 0.0.0.0:15000` and put the server machine's reachable LAN address in the client argument.

This direct path bypasses the retired EA/GILS sign-in and launch service. The local key is only a development connection value; it is not an EA credential, and the emulator does not validate EA accounts or tickets. The client launcher and game files remain in a separate local client workspace and are not part of this server repository.

The persistent campaign file defaults to `runtime/world.json` (`-world`). Text packet traces default to `runtime/traces/` (`-trace-dir`); each connection creates a `session-<timestamp>.log` file. The current isolated trial uses `runtime/world-test-15011-combat-fire.json` and `runtime/traces/native-combat-fire/`.

## Aries transport

Each TCP frame begins with 12 bytes:

| Offset | Size | Meaning |
| --- | ---: | --- |
| 0 | 4 | Message type, little-endian |
| 4 | 4 | Tag, little-endian |
| 8 | 4 | Payload length, little-endian |
| 12 | length | Payload, at most 1 MiB |

The server sends transport type `0x16` (`LOGIN_REQUEST`) after accepting a connection. A client `0x15` (`LOGIN`) receives the game host greeting inside a data frame. Type `0x00` is a data frame; type `0x03` is a redirect frame. Other transport types are logged as unimplemented.

## Game PDU framing

A game PDU is carried in an Aries data-frame payload. Its six-byte header is big-endian: a 16-bit message ID followed by a 32-bit total PDU length, including the header. Multiple PDUs may be decoded from one data payload. The client buffer limit used by the server is `0x4000` bytes.

For oversized messages the server emits `SPLIT_BUFFER_PDU` (`0x47`) wrappers containing a final-fragment byte, a 32-bit byte count, and a chunk of the original encoded PDU. The serializer shape is recovered; complete client-side reassembly behavior is still uncertain.

## Current connection flow

1. A TCP connection receives `LOGIN_REQUEST`; a client `LOGIN` receives `HOST_ONLINE` (`0x1D`).
2. `CLIENT_ONLINE` (`0x09`) receives the local player record (`0x3C`), profile (`0x3D`), and an unsynchronized static-configuration update (`0x86`). The client’s `STATIC_CONFIG_RESPONSE` (`0xA4`) completes the sync exchange.
3. Startup settings requests are answered with their default response forms. A `CREATE_CHARACTER` (`0x87`) request creates or loads the local pilot and receives the current zero-result completion form (`0x88`). The authentic success semantics of that response remain unconfirmed.
4. A `JOIN_PLANET` (`0x91`) request changes the pilot’s planet location. Live connected occupants are announced with `OCCUPANT_UPDATE` (`0x94`) and arrival/departure messages (`0x2B`/`0x2C`). The server answers a rejected join with `JOIN_PLANET_FAILED` (`0x92`).
5. `LIST_ROOMS` (`0x26`) receives an empty `LIST_ROOMS_RESPONSE` (`0x27`). Solaris staging rooms are maintained from world state and, when enabled, sent as room-list/add/update/remove messages (`0x97`–`0x9A`). A mission join uses `JOIN_ROOM` (`0x23`); leaving can use `EXIT_ROOM` (`0x17`) or the client’s parent-city rejoin path.
6. `READY_STATE_CHANGE` (`0x8C`) requests update the selected Mech or readiness state. When the room is ready, the scheduler publishes a countdown and the clients enter combat together.
7. `COMBAT_DATA` (`0x81`) streams are parsed for supported native position and fire commands. Those commands are relayed to peers in the same active room. This is peer relay; the server does not currently calculate hits, damage, destruction, or match results.

## Implemented message addresses

| ID | Name | Current server use |
| ---: | --- | --- |
| `0x08` | `CHAT_MESSAGE` | Relays supported All, Team, and House channels to matching live sessions. Team channel value 1 is inferred; House is 2 and All is 0. |
| `0x09` | `CLIENT_ONLINE` | Starts player/profile/static-configuration bootstrap. |
| `0x17` | `EXIT_ROOM` | Removes the pilot from the current mission and returns an acknowledgement form. |
| `0x1D` | `HOST_ONLINE` | Server greeting after login. |
| `0x23` | `JOIN_ROOM` | Joins a local Solaris mission or re-enters its parent staging room. |
| `0x26` / `0x27` | `LIST_ROOMS` / response | Returns the current empty global room list. |
| `0x2B` / `0x2C` | occupant arrived / departed | Updates live occupant presence. |
| `0x31`, `0x33`, `0x35`, `0x37`, `0x43` | settings requests | Receives the corresponding default-form setting response. |
| `0x3C` / `0x3D` | player / profile update | Sends local pilot and profile records during bootstrap. Peer rosters use occupant messages instead. |
| `0x3E` | room update | Optional recovered Solaris lobby room update, gated by `-experimental-planet-room-type=1` or `2`. |
| `0x47` | split buffer | Wraps oversized outgoing game PDUs. |
| `0x81` | `COMBAT_DATA` | Parses supported setup, position, and fire command streams; only the supported position/fire events are relayed. |
| `0x86` / `0xA4` | static configuration update / response | Completes startup configuration synchronization. |
| `0x87` / `0x88` | create character / reply | Handles a local call sign and returns the currently experimental zero-result completion form. |
| `0x8B` | combat configuration update | Publishes current mission seat and combat configuration. |
| `0x8C` / `0x8E` | ready request / seat update | Handles Mech selection and ready toggles; emits seat updates. |
| `0x90` / `0x91` / `0x92` | combat summary / join planet / join failure | `0x91` changes planet; `0x92` reports a rejected join. `0x90` is not used as a join reply. |
| `0x94` | occupant update | Publishes pilot data for a live roster entry. |
| `0x97`–`0x9A` | ready-room list / added / update / removed | Maintains optional Solaris staging-room lists. |
| `0x9E` / `0xA9` / `0x80` | current combat / countdown / enter combat | Sends mission state, launch timing, and combat entry. |
| `0xA7` / `0xA8` | request / planet state list | Answers the global map poll with current House ownership, unless an experimental probe is enabled. |
| `0xAD` | request lobby rooms | Sends the current empty auxiliary room list. |

Unknown game PDUs are ignored except for the recognized startup-setting request IDs, which receive default-form responses. This is a deliberately small local emulator, not a complete recreation of the legacy online service.

## Room counts and sides

Live planet occupants are tracked by connected sessions, not by the saved pilot list. Mission occupancy and open seats come from the room’s actual seats, including AI seats. The Solaris room encoder places defender-seat values in the Alpha-facing field and attacker-seat values in the Beta-facing field, based on the current row-renderer observation.

The user has reported that some room counts, displayed occupants, or team names may still be incorrect. Treat this area as under investigation until join, leave, disconnect, reconnect, and both-side assignment cases have been checked against packet traces and client display. The server-side count alone does not prove that the client renders the right value.
