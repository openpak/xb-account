# Notes: xb-account

Original Xbox identity adapter for OpenPak. This independently built service
calls account core's existing gRPC Links and Identity contracts. It never reads
core tables or exports passwords, Kerberos keys, email or web sessions.

## Principals, provisioning and revocation

State lives in an embedded SQLite file (`XB_DB_PATH`, pure-Go modernc driver,
WAL). Chosen over Postgres because the state is small and single-writer, it
needs no new database/role in production, and the image stays CGO-free. The
core's Postgres is never shared. `XB_PEPPER_FILE` holds >=32 bytes of hex used
to key the device-proof MAC (proofs are never stored).

## Repository

- Provenance and licence: AGPL-3.0-only, see `LICENSE` and `NOTICE`. Derived from the Xombie project in part; the legacy Rust workspace remains in https://github.com/openpak/xbox-live.
