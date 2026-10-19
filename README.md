# Abi · Una aventura vikinga

Spanish family invitation for **xitlaliabigail.com**, with RSVP, private reports and a sending tracker. Go Lambda + DynamoDB, static S3/CloudFront frontend, Pulumi infrastructure with private S3 state. No external email/SMS service.

Companion: [birthday.invitation.android](https://github.com/lambdawalker/birthday.invitation.android). Messages are sent through the phone SIM or handed to WhatsApp for a human to send.

## Screens
- `/`: invitation. Family links use `/?i=...`; an unpersonalized visit shows event information but cannot RSVP.
- `/admin.html`: attendance and headcounts; export CSV.
- `/send.html`: sending progress, assignments, history, manual SMS/WhatsApp tools and app users with single-use sign-in PINs.

Event: October 24, 2026, **4 p.m. America/Ciudad_Juarez**. Includes the weather-related change from 5 p.m., gift preferences, map and parking directions. Neighborhood name omitted until confirmed. Parking tab searches Calle Ingeniería; it does not invent an exact parking-lot coordinate. Check the map with the host before sharing.

## Local development
Install Go (version from `go.mod`; automatic toolchain download is supported).

PowerShell:
```powershell
$env:ADMIN_KEY = 'local-development-key-at-least-32-characters'
go run ./cmd/server
```
Bash:
```bash
ADMIN_KEY=local-development-key-at-least-32-characters go run ./cmd/server
```
Open `http://localhost:8080/?i=0123456789abcdef0123456789abcdef`.
The local database is in memory and resets on restart. The admin key above is only an example for localhost. To use DynamoDB locally, set `TABLE_NAME`, `AWS_PROFILE` and `AWS_REGION`.

## Deploy from Windows or Linux
Prerequisites: Go (version in `go.mod`), AWS CLI v2, and Pulumi CLI (compatible with the pinned Go SDK). Python, Node.js and npm are not required. Install from their official sites. You do **not** need a Pulumi account.

1. Configure AWS SSO with `aws configure sso` and select a role permitted to create the stack resources.
2. Add the domain to Cloudflare and activate its nameservers. Obtain a token scoped to that zone with **DNS Edit** and **Zone Read**.
3. Run `deploy.bat` on Windows (PowerShell: `.\deploy.bat`) or `./deploy.sh` on Linux from the repository root. Both launch the Go wizard and also work when invoked using their full path from another directory.
4. The wizard checks tools, logs in with AWS SSO when needed, creates/uses an encrypted versioned private S3 state bucket, asks for a Pulumi encryption passphrase, and requests the Cloudflare token without echoing it.
5. It generates encrypted admin/import secrets, compiles the Lambda, compiles the Go infrastructure, and runs `pulumi preview`. Press Enter to accept `DEPLOY` after reviewing the preview, or type `CANCEL` to stop. State bootstrap defaults to `CREATE`; Enter creates the bucket if it is missing.
6. Save the passphrase in your password manager; use the same passphrase and state bucket for later updates. Do not rotate `importNamespace` between imports.
7. From `infra`, use `pulumi config get adminKey` in a private terminal to obtain your administrator key. Open `/admin.html`.

The wizard temporarily holds Cloudflare and Pulumi secrets in its process environment. No cloud credentials or guest CSVs belong in Git. Stack YAMLs are ignored. Never commit real exports. DynamoDB deletion protection and Pulumi protection are enabled; deleting a stack requires an intentional separate removal of those protections.

CloudFront uses DNS-only Cloudflare CNAME records (including apex flattening), AWS ACM in us-east-1, a private S3 origin, HTTPS API origin, secure headers and no API caching. API Gateway is throttled. Terraform-style resource replacements can still be billed: review preview before applying. State bucket is deliberately outside stack lifecycle.

Static files cache for five minutes. After an urgent update you can invalidate with:
```bash
aws cloudfront create-invalidation --distribution-id YOUR_DISTRIBUTION_ID --paths '/*'
```

## Import families
CSV headers: `name,phone,email` (email may be blank), optional `id` (32 lowercase hex characters). Use E.164 numbers: Mexico `+52` plus ten digits. One row per family. Example: `examples/guests.csv`.

Set `IMPORT_NAMESPACE` to `pulumi config get importNamespace` and `TABLE_NAME` to `pulumi stack output tableName` (run these from infra). Keep the namespace private; it derives stable unguessable family IDs from phone numbers.
```bash
go run ./cmd/import-guests -file my-guests.csv          # validate only
go run ./cmd/import-guests -file my-guests.csv -apply   # create missing families
```
Existing records are never overwritten; RSVPs, sending and assignments are preserved. Repeating an import with the same namespace/phone is safe. Changing a phone changes derived ID unless an explicit original `id` is retained. The CLI does not update existing contacts; correct/import before sending or use a reviewed database maintenance operation.

## Connect Android
In `/send.html` or `/admin.html`, use **App users** to create a username (3–40 letters, numbers, dots, underscores or hyphens). Share the username and generated eight-digit PIN privately. The app defaults to `https://xitlaliabigail.com`; the user enters their username and PIN, with no credential copying. PINs expire after 15 minutes, allow at most five incorrect attempts, and work once. The app stores the resulting session using Android Keystore.

Use **Generate new PIN** beside an existing username after logout, reinstall, expiry, or lockout. This immediately invalidates the old PIN and app session while preserving family assignments and sending history. **Revoke access** disables both the session and outstanding PIN; generating another PIN re-enables the user. One active app session per username is supported. Stop sending on the old phone before replacing its session. Existing legacy device credentials still work and remain revocable under **Previously connected devices**.

Deploy the backend/web changes first with the usual launcher, then install Android version 1.1.0. No table recreation or new AWS permissions are needed: user records use the existing DynamoDB table with conditional writes. Existing legacy assignments stay with their old device; release them in the web report before claiming them under a new username. For users created with PINs, re-login preserves their assignments automatically.

Assign families before sending; assignments appear in both reports. Keep the app open for the paced batch and test one recipient first. Replacing access cannot undo an SMS already handed to Android. If a sign-in response is lost, generate another PIN; it may already have been consumed.

## Go commands and verification

All commands run from the repository root:

```bash
go run ./cmd/server         # local invitation and API; set ADMIN_KEY first
go run ./cmd/build          # build/lambda.zip, Linux arm64 on any host
go run ./cmd/deploy --check # check prerequisites, no cloud changes
go run ./cmd/deploy         # same wizard as deploy.bat / deploy.sh
go test ./...
go vet ./...
```

The root Go module contains `cmd/server`, `cmd/lambda`, `cmd/import-guests`, `cmd/build`, `cmd/deploy`, `internal/app`, `internal/deploytools`, and the Go Pulumi program in `infra/`. The browser assets in `web/` remain HTML/CSS/JavaScript; they are served directly and need no Node build. The Android companion remains in its separate repository.

CI runs tests, vet, Lambda packaging and deployment/infrastructure compilation on Windows and Linux; Linux also runs race detection. Pulumi mock tests exercise the resource graph without cloud credentials. They do not substitute for reviewing `pulumi preview` against your AWS account. No deployment is performed by CI.

The Go infrastructure uses Pulumi’s core `RegisterResource` API with pinned AWS/Cloudflare provider versions, preserving the existing resource types, logical names, configuration keys and exports. Existing stacks keep their S3 backend and passphrase: do not create a replacement stack or rotate importNamespace. Review the preview for unexpected replacements before accepting DEPLOY. Old Python and TypeScript tooling has been removed.

Hidden prompts require an interactive terminal. For an existing shell-managed setup, the wizard can also read `PULUMI_CONFIG_PASSPHRASE` and `CLOUDFLARE_API_TOKEN` from the environment. Never paste these into command-line arguments or commit them. The deployment wizard still asks you to review and approve the preview.

AWS provisioning, carrier delivery and physical Android permissions require your environment. See the [API contract](docs/api.md). The earlier design and plan under `docs/superpowers/` are historical; this README documents the current Go layout.

Developer tooling, deployment prompts and diagnostics are in English; the guest invitation remains in Spanish. Machine-read AWS commands explicitly request JSON output, so profiles configured for text, table or YAML do not need to be changed.

### Deployment prompt defaults

Every non-secret prompt accepts Enter: AWS profile (`AWS_PROFILE` or `default`), region (`us-east-1`), state bucket (`abi-pulumi-<account>-<region>`), bucket creation (`CREATE`), stack (`production`), domain (`xitlaliabigail.com`), Cloudflare Zone ID (`CLOUDFLARE_ZONE_ID` or `auto`), and final deployment (`DEPLOY`). Enter `CANCEL` at either confirmation to stop.

The `auto` Zone ID default looks up the exact domain using your Cloudflare token; you can enter the Zone ID manually instead. Tokens and the Pulumi encryption passphrase have no default and remain hidden, or can be supplied through their existing environment variables.


Edit [deploy.defaults.yaml](deploy.defaults.yaml) in the repository root to customize these non-secret defaults. Both launchers load it automatically, regardless of the directory you launch from. Omitted fields retain built-in defaults; a missing file also uses built-in defaults. Invalid or unknown fields fail before any cloud commands run.

Typed answers override the displayed defaults. For the AWS profile and Cloudflare Zone ID, environment variables override the YAML defaults. The state bucket supports `{account}` and `{region}` placeholders, expanded after AWS login and region selection. Set either confirmation default to `CANCEL` if you prefer Enter to stop. Pulumi continues to use the existing `infra` project directory; no empty project directory is needed. Keep all tokens and the encryption passphrase outside this file.


If Pulumi offers to create a new project or asks for an empty directory after login, cancel with Ctrl+C and update the repository before rerunning the launcher. The wizard runs all Pulumi commands with `--non-interactive`, uses the existing `infra` directory, and creates/selects the configured stack itself. Keep your existing state bucket and encryption passphrase.


If an update fails with CloudFront `NoSuchCachePolicy`, update to the corrected infrastructure code and rerun the launcher with the same AWS account, state bucket, stack, and encryption passphrase. The API behavior uses AWS's managed CachingDisabled policy (`4135ea2d-6df8-44a3-9df3-4b5a84be39ad`). Pulumi retains resources successfully created before a failure and reconciles them on the next update; do not delete the stack or recreate the bucket to retry.
