# Shared API v1

All routes are same-origin `/api`. JSON UTF-8. Errors: `{ "error": "message" }` (new user authentication messages are English). 400 invalid data, 401 invalid/expired credentials, 403 scope/origin, 404 missing invitation, 409 conflict, 503 transient persistence error. Do not blindly retry sends after 409/timeouts.

## Public
- `GET /api/invitations/{id}`: `{name,rsvp,adults,children}` only; no contact information.
- `POST /api/invitations/{id}/rsvp`: `{rsvp:"yes"|"no",adults:int,children:int}`. No arbitrary family-size limit. Nonnegative integers, at least one person for yes. No resets counts to zero.
- Invitation URLs: `https://xitlaliabigail.com/?i={128-bit-id}`. These are bearer links; keep private.

## Administrator
- `POST /api/login` with `{key}` exchanges deployment key for a signed 12-hour HttpOnly Secure SameSite=Strict cookie. POST requests from other origins rejected. Local localhost development uses a non-Secure cookie.
- `POST /api/logout` clears cookie.
- `GET /api/admin/families` → `{families:[Family],baseUrl}`.
- `GET /api/admin/devices` → device metadata (no raw tokens).
- `POST /api/admin/devices` with `{name}` → `{id,name,token,baseUrl}`. Raw token shown once. Server stores SHA-256 hash only.
- `DELETE /api/admin/devices/{id}` revokes token. Ownership stays visible; administrator reconciles in-flight attempts before release.

## App users and single-use PINs
Administrator authentication is required for all `/admin/users` routes:
- `GET /api/admin/users` → `[{id,username,status,pinExpires}]`, without PIN/session hashes.
- `POST /api/admin/users` with `{username}` → `{user,pin,baseUrl}`. Usernames are case-insensitive, trimmed and unique; 3–40 ASCII letters/digits/dots/underscores/hyphens, starting with a letter or digit. Duplicate names return 409.
- `POST /api/admin/users/{id}/pin` → `{user,pin,baseUrl}`. Replaces any previous PIN and session, restores revoked access, and preserves the user's stable assignment ID.
- `DELETE /api/admin/users/{id}` revokes both PIN and session; assignments are retained.

Public sign-in:
- `POST /api/user/login` with `{username,pin}` → `{token,id,username,baseUrl}`.
- PIN is an eight-digit string (preserve leading zeros), expires in 15 minutes, and is consumed atomically. Five incorrect valid-format attempts lock it until the administrator generates another PIN. Invalid, expired, consumed, locked and unknown-user attempts return the same 401. Contention may return 429; storage errors return 503. Do not automatically retry a successful/uncertain sign-in: ask for a new PIN if the response was lost.
- `POST /api/user/logout` with the user bearer token invalidates its session; sign-in requires a newly generated PIN.

PIN verifiers are HMAC-SHA256 using the backend secret; only a SHA256 session hash is stored. Conditional revisions enforce single use and failed-attempt limits across Lambda instances. Sessions persist until logout, revocation or PIN regeneration. Each user has one active session. The PIN is never returned by list routes.

## Android device
New sign-ins use `Authorization: Bearer user_<stable-id>.<random-secret>`. Use the returned `id` (also the token's stable-id segment) for ownership checks, not a hash of the full session token. Legacy `device_...` credentials remain supported with their original SHA256 identity.
- `GET /api/families` → `{families:[Family],baseUrl}`.
- `POST /api/families/{id}/actions`: `{action,channel?,attemptId?,status?,detail?}`.

Actions:
- `claim`: conditionally assign if free or already owned by caller.
- `release`: remove assignment; cannot release `sending` until reconciled.
- `begin`: channel `sms` or `whatsapp`, new UUID attemptId. Changes pending/failed to sending/whatsapp_opened. Repeating the same ID returns state, **not permission to transmit again**. Persist a local intent before request and dispatch state before OS send.
- `result`: same attemptId; `sent`, `delivered` (SMS), `manual_sent` (either), `failed`, `uncertain`. Completed states cannot be downgraded. Exact current-state duplicate is idempotent. Never treat opening WhatsApp as sent.
- `manual` (admin): `{channel}` records human-confirmed sending.
- `reset` (admin): requires nonempty detail, no active `sending`; clears current attempt and returns pending. History retained. Verify phone stopped before reset; this is explicit authorization to prepare another attempt.

A device must own the family for actions except claim. Admin may reconcile/release device-owned records, but cannot silently send manually over an assignment. All family mutations are revision-conditional. On 409 refresh and re-evaluate; do not resend SMS.

Family: `{id,name,phone,email?,rsvp,adults,children,status,channel?,assignedTo?,assignedName?,attemptId?,updatedAt?,revision,history:[{id,status,channel,at,actor,detail?}]}`.

SMS delivery is not exactly-once across the carrier boundary. A process crash can leave an uncertain result. Keep it visible and require human reconciliation. Device revocation prevents future backend calls; it cannot cancel an SMS already handed to Android.


## Add families from selected contacts
Authenticated app users, legacy devices, and administrators may call:
`POST /api/families/import` with `{contacts:[{name,phone,email?}]}`.
Maximum 20 entries and 32 KiB JSON per request; larger selections must be chunked. Unknown fields are rejected. The server requires international phone numbers, strips common formatting, accepts 00 prefixes, and normalizes legacy Mexican +521 numbers to +52. Local-country assumptions belong in the reviewed app UI, not the server.

Response: `{results:[{index,name,phone,status,familyId?,message?}]}`, where status is `added`, `existing`, `invalid`, or `error`. A successful HTTP 200 may contain per-contact errors. `index` refers to the original request position. `familyId` may be absent for a duplicate detected in a concurrent transaction. Anonymous/revoked access returns 401; malformed or oversized batches return 400; an initial roster lookup failure returns 503 before writes begin.

New rows have random 128-bit invitation IDs, pending RSVP/sending state, and `createdBy`/`createdName` metadata. New rows are assigned to the authenticated importer (administrator-created rows remain unassigned). Existing rows are never overwritten or reassigned. Family creation and a `P#<SHA256(normalized phone)>` reservation are one DynamoDB transaction with conditional puts. Transient transaction contention is reported as error, not as a duplicate.

Retries are safe by normalized phone identity, including after response loss or partial completion. The endpoint never sends messages. Updated CSV import uses the same reservation protocol; old standalone import binaries should be replaced during rollout.
