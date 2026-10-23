# Data

`data.go` provides optional seed loading with a synthetic fallback. Operator-supplied campaign seed tables remain external and are selected with `MPBT3025_SEED_DIR`.

[`test-fixtures/`](test-fixtures/) contains generated JSON fixtures for native combat command tests. These fixture files are test evidence and are not campaign seed tables.

`combat-native-targeted-fire.json` and `combat-native-server-targeted-fire.json` are synthetic outputs from the pinned native writers with explicit weapon/target overrides. `combat-live-standalone-fire.json` contains only the protocol bytes of one October 23 untargeted fire event, used to reproduce the old position-first parser's dropped-shot bug; it contains no client binary or art.
