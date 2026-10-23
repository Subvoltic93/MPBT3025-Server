# Data

`data.go` provides optional seed loading with a synthetic fallback. Operator-supplied campaign seed tables remain external and are selected with `MPBT3025_SEED_DIR`.

[`test-fixtures/`](test-fixtures/) contains generated JSON fixtures for native combat command tests. These fixture files are test evidence and are not campaign seed tables.

`combat-native-targeted-fire.json` and `combat-native-server-targeted-fire.json` are synthetic outputs from the pinned native writers with explicit weapon/target overrides. `combat-live-standalone-fire.json` contains only the protocol bytes of one October 23 untargeted fire event, used to reproduce the old position-first parser's dropped-shot bug; it contains no client binary or art.

`combat-live-targeted-fire.json` preserves two opposite-direction native laser reports (position/fire batches) from the controlled 200 metre trial. `combat-native-damage.json` and `combat-native-local-damage.json` execute the pinned DLL’s separate damage writers. They establish command bytes and envelope behavior; no damage command is emitted by the server yet.

`combat-native-armor-resolution.json` records original native armor resolution and assignment against synthetic arrays. It declares the substituted recorder/getter; no live damage, destruction, or client artwork is included. Regenerate with `python3 scripts/native-damage-fixture.py`.

`combat-live-armor-probe.json` contains two native client reports, their server assignment/fire streams observed in both session traces, and a later laser report with no additional assignment. Both clients showed matching torso damage after each diagnostic update. It contains protocol bytes only.
