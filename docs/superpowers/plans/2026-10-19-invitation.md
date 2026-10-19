# Invitation Platform Implementation Plan

> For agentic workers: use superpowers:executing-plans task-by-task.

**Goal:** Deliver deployable invitation, admin tools and compatible Android sender.
**Architecture:** Static Spanish site plus shared Go HTTP/Lambda API, DynamoDB conditional records, Android authenticated API client.
**Tech Stack:** Go, vanilla browser JavaScript, Pulumi TypeScript, Python deployment CLI, Kotlin/Compose Android.
**Spec:** ../specs/2026-10-19-invitation.md

## Global Constraints
- No external messaging providers; no deployment or real sending during development.
- Event 2026-10-24 16:00 America/Ciudad_Juarez; unlimited family size.
- Keep guest data and secrets out of source control; distinct assignment, sending and RSVP states.

## Review Focus
- Racing device claims must produce one owner (Go tests).
- Replayed sending results must be idempotent and preserve uncertain state (Go/Android tests).
- Revoked credentials must fail (Go tests).
- App crash after carrier accepts message must never auto-resend (queue tests).
- Invalid public RSVP must not expose contacts or corrupt stored response (Go tests).

## Task 1: Shared backend and contract
Files: backend/internal/app/{model,store,handler,auth}.go, *_test.go; backend/cmd/{server,lambda,import-guests}; docs/api.md.
Interfaces: JSON Family and POST actions, random 128-bit family ID; revision CAS.
- [ ] Write failing request tests for auth, RSVP validation/privacy and concurrent claim/attempt actions.
- [ ] Run go test ./... and observe missing implementation failures.
- [ ] Implement handlers, in-memory/dynamo repositories, auth and CSV CLI.
- [ ] Run go test -race ./... and build Linux arm64 Lambda; commit.

## Task 2: Browser interfaces
Files: web/{index,admin,send}.html, web/{app,admin,styles}.js/css, web/assets.
Consumes: Task 1 API. Produces public RSVP, admin report, manual send tracker, device pairing/revocation.
- [ ] Implement invitation with provided optimized artwork and accurate map links.
- [ ] Implement authenticated admin report and send controls with errors and status distinctions.
- [ ] Run browser smoke tests and inspect desktop/mobile screenshots; commit.

## Task 3: Infrastructure and deployment
Files: infra/{index.ts,package.json,Pulumi.yaml}, scripts/{deploy,build}.py, workflows.
Consumes Lambda ZIP and web files. Produces TLS CloudFront/S3/API/Dynamo stack and DNS.
- [ ] Declare least privilege resources and S3 backend wizard.
- [ ] Typecheck infrastructure, test deployment helpers, verify ZIP contents; commit.

## Task 4: Android companion (separate repo)
Files: app Kotlin source, Gradle files, docs/api.md; unit tests.
Consumes Task 1 API. Produces claims, SMS begin/results and manual WhatsApp confirmation.
- [ ] Write queue rules tests; verify failures before implementing policy.
- [ ] Implement pairing, persistent local state/outbox, SMS callbacks, WhatsApp handoff, Compose UI.
- [ ] Build/test/lint or report exact environment blockers; commit.

## Task 5: Review and delivery
- [ ] Review both branches, run all available checks, repair material issues.
- [ ] Push feature branches and open PRs with exact validation and deployment limitations.
