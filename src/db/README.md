# Database placeholder

This directory reserves a home for a future persistence adapter and schema. It is a scaffold only; the server does not import this package or open a database.

Current persistence is JSON-backed and remains in `src/state`. Keep future database work behind a clear adapter boundary so protocol handlers do not depend directly on a storage driver. No database engine, migration format, or schema has been selected.

`schema.sql` is an empty design placeholder. Add tables only after the storage backend and migration approach are chosen.
