# Permanent account handles

GetAccount.handle is the canonical gamertag, used verbatim for new automatic
identities. It is independent of the editable display name. A canonical handle
collision fails; it never selects a suffix or another name. Legacy accounts
without a handle keep the old seed during the coordinated alpha migration.

Provisioning and console signup require the gamertag to equal the chosen core
handle. SetGamertag remains an internal applier for the reviewed alpha migration:
once core owns a handle it can only apply that exact value, even when forced.
The website no longer exposes a rename form. Existing console-bound checks and
audit records remain; the approved console profile must be re-enrolled after a
migration that changes its sign-in name. XUID and credentials are not recreated.

Core owns shared format, reserved-name and profanity policy. Its database also
prevents public-code drift. Legacy reserved-name checks only serve pre-migration
accounts. Deletion already releases local gamertag aliases; the core retains the
handle until 90 days after permanent deletion, so that local release cannot make
it available early. Automatic identities continue to carry no credentials.
