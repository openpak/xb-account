# Protocol: xb-account

`POST /internal/v1/resolve` accepts `{"gamertag":"Player One"}` and returns
`{"xuid":"ACCOUNT_ID","account_id":"…","display_name":"…"}` only for
an active `xbox` link and active account. Provisioning must publish the native
gamertag as that link's public code and use a canonical 16-character lowercase
hexadecimal XUID as its subject. This lookup does not prove possession of a
native credential: xb-auth/xb-gateway must validate native tickets separately.
Machine/device principals and provisioning are not implemented by this lookup.

The server requires TLS 1.3 and a client certificate signed by its dedicated CA,
with URI SAN `spiffe://openpak/xb-auth` or `spiffe://openpak/xb-gateway`.
`GET /livez` uses the same mutual TLS and only reports listener liveness.
No readiness or successful native-sign-in claim follows from that endpoint.

Every lookup has a three-second total core deadline, checks account status, and
rechecks its link before returning. No positive identity cache is used. Core
unavailability returns 503; unknown/revoked identity 404; disabled account 403.
Requests and core responses are never logged. In-flight sessions still require
future invalidation/event handling; this lookup only governs admission.

## Principals, provisioning and revocation

Distinct principal spaces: user (XUID, link namespace `xbox`, never `xbl`) and
device (machine ID). One primary Xbox user per core account; gamertags are
unique, case-insensitive aliases released on unlink/delete.

Contract `xb-common/xbaccountapi` (mTLS; xb-auth and xb-gateway): `ResolveUser`,
`ResolveGamertag`, `ResolveDevice`, and the `Revocations` poll stream. Each
resolve re-reads core (link + account status) and denies banned, unlinked,
revoked, deleted or unavailable. Revocations are a durable outbox fed by the
core event cursor (`account_banned/unbanned`, `link_unlinked`,
`account_delete_started/restored/deleted`, `credential_revoked`), idempotent by
event id. An unlink also revokes the account's devices. A closed account is
suspended for the core's grace period (denied as revoked, gamertag held),
comes back with the same gamertag on `account_restored`, and only
`account_deleted` (final erasure) releases the gamertag.

Provisioning (`xb-provisioner` begins, `xb-auth` finalizes): web approval mints
a 10-minute single-use token bound to a core account, machine ID and a
device-generated challenge. Finalize resumes/replays safely: mint XUID,
ReserveLink, ActivateLink, SetLinkPublicCode(gamertag), local commit, with
deterministic idempotency keys; terminal failures UnlinkLink and burn the token.

## Device-pairing enrolment and subscription codes

`internal/enrol` and `internal/voucher` add enrolment without the dashboard
supplying the machine key: a console tool registers a pending enrolment through
`xb-enroll` (SPIFFE `spiffe://openpak/xb-enroll`, two routes only), the website
approves it for a core account (SPIFFE `xb-provisioner`), and the device is
linked (ReserveLink/ActivateLink, XUID reserved, no gamertag yet). Subscription
codes are issued by the website and consumed by `enrol.CompleteSignup`. Routes,
states, limits and error codes: internal research notes.
The service-5 wire handling for verifyvoucher/createowneracct is not implemented.

## Verification

Generic password verification is not Kerberos proof. Neither a core password check, a provisioning token, nor the stored device proof MAC authenticates a native Kerberos exchange; xb-auth must validate pre-authentication itself. This service calls account core's gRPC Links and Identity contracts and never reads core tables or exports passwords, Kerberos keys, email or web sessions.

## Offering calls (`/xbos/`, native services 4 and 9)

Served over the gateway feature stream next to the account calls
(`internal/svc5`). Wire formats and RE addresses: `research/xbox/xbos-protocol.md`
in the private research repo.

| Path | Service (`Content-type`) | Request | Reply |
|---|---|---|---|
| `/xbos/OfferingDetailsNoUser.ashx` | 4 (`xon/4`) | 27 bytes: country, machine id, u32, offer id, u32, u16 | offer record, X-Err 0 |
| `/xbos/OfferingDetails.ashx` | 4 | 34 bytes: XUID 8, machine id 8, u32, offer id 8, u32, u16 | the same offer record; unknown offer id 0x80154007; XUID of another account 0x8015400a |
| `/xbos/OfferingCancel.ashx` | **9** (`xon/9`) | 16 bytes: XUID 8, offer id 8 | X-Err 0, empty body (OpenPak has no paid subscription; success changes nothing) |

Cancel arrives as service 9: the gateway needs an `XBGW_BACKENDS` entry `9=` for
it. `/xbos/verifynickname.ashx` (service 9) is not called by any stock 5960
dashboard code and stays 501. `/xbos/AutoupdReferral.ashx` is service 3 and is not
served here (see `xb-update`).
