# Changelog

## v0.2.0 - 2026-11-04

- Account erasure (card #120). `DELETE /internal/accounts/{account_id}/data` (bearer `ACCOUNT_ERASE_KEY`, listener `ERASE_ADDR`, default `:9099`, container network only, not served while the key is unset) deletes in one transaction the account's users, gamertag aliases and audit, user settings, auto identity, gamertag reservation, devices (console ids), provisions, enrolments, vouchers and revocation entries; the gamertag is released.
- The `account_deleted` core event now runs the same erase (it only released the gamertag and kept the rows before). The deletion revocation entry (account id and reason only) is still appended so the gateways drop live sessions. Behaviour change: a deleted account now resolves as unknown instead of `deleted`; both deny.

## v0.1.39 - 2026-11-03

- State moved from the embedded SQLite file to Postgres (schema `xb_account` of the shared `xbox` database). `XB_ACCOUNT_DATABASE_URL` is required; `XB_DB_PATH` and the modernc SQLite driver are gone.
- Existing data is copied into Postgres by a one-off import; the old SQLite file is left untouched on the volume.
- Tests run against Postgres (`XB_TEST_DATABASE_URL`) and cover a restart on the same database.
