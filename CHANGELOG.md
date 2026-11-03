# Changelog

## v0.1.39 - 2026-11-03

- State moved from the embedded SQLite file to Postgres (schema `xb_account` of the shared `xbox` database). `XB_ACCOUNT_DATABASE_URL` is required; `XB_DB_PATH` and the modernc SQLite driver are gone.
- Existing data is copied into Postgres by a one-off import; the old SQLite file is left untouched on the volume.
- Tests run against Postgres (`XB_TEST_DATABASE_URL`) and cover a restart on the same database.
