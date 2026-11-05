# Changelog

## v0.3.0 - 2026-11-05

- The gamertag is a cache of the core handle (data duplication card #131, phase 6). A new sync (internal/gamertag) re-reads the account on every core `profile_updated` event (push) and runs a full pass at start and every 5 minutes (backup TTL). A row that differs from the core handle is repaired, core wins: users.gamertag, the alias (name-reservation table), the core link public code, plus a `gamertag_audit` row (actor `core-handle-sync`). A handle held by another identity is never taken (counted as failed). Core unreachable: the cache is served unchanged. One log line with counts only.
- Erasure unchanged and covered (audit rows written by the sync are deleted by the same account erase; test added).

## v0.2.0 - 2026-11-04

- Account erasure (card #120). `DELETE /internal/accounts/{account_id}/data` (bearer `ACCOUNT_ERASE_KEY`, listener `ERASE_ADDR`, default `:9099`, container network only, not served while the key is unset) deletes in one transaction the account's users, gamertag aliases and audit, user settings, auto identity, gamertag reservation, devices (console ids), provisions, enrolments, vouchers and revocation entries; the gamertag is released.
- The `account_deleted` core event now runs the same erase (it only released the gamertag and kept the rows before). The deletion revocation entry (account id and reason only) is still appended so the gateways drop live sessions. Behaviour change: a deleted account now resolves as unknown instead of `deleted`; both deny.

## v0.1.39 - 2026-11-03

- State moved from the embedded SQLite file to Postgres (schema `xb_account` of the shared `xbox` database). `XB_ACCOUNT_DATABASE_URL` is required; `XB_DB_PATH` and the modernc SQLite driver are gone.
- Existing data is copied into Postgres by a one-off import; the old SQLite file is left untouched on the volume.
- Tests run against Postgres (`XB_TEST_DATABASE_URL`) and cover a restart on the same database.
