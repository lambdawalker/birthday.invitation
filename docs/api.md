# Shared API v1

All routes are same-origin `/api`. JSON UTF-8. Errors: `{ "error": "Spanish message" }`. 400 invalid data, 401 invalid/expired credentials, 403 scope/origin, 404 missing invitation, 409 conflict, 503 transient persistence error. Do not blindly retry sends after 409/timeouts.

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

## Android device
Use `Authorization: Bearer device_...`. No AWS credentials. Device ID is lower-case SHA-256 of credential.
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
