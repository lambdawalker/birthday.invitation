# Abi · Una aventura vikinga

Spanish family invitation for **xitlaliabigail.com**, with RSVP, private reports and a sending tracker. Go Lambda + DynamoDB, static S3/CloudFront frontend, Pulumi infrastructure with private S3 state. No external email/SMS service.

Companion: [birthday.invitation.android](https://github.com/lambdawalker/birthday.invitation.android). Messages are sent through the phone SIM or handed to WhatsApp for a human to send.

## Screens
- `/`: invitation. Family links use `/?i=...`; an unpersonalized visit shows event information but cannot RSVP.
- `/admin.html`: attendance and headcounts; export CSV.
- `/send.html`: sending progress, assignments, history, manual SMS/WhatsApp tools and revocable device credentials.

Event: October 24, 2026, **4 p.m. America/Ciudad_Juarez**. Includes the weather-related change from 5 p.m., gift preferences, map and parking directions. Neighborhood name omitted until confirmed. Parking tab searches Calle Ingeniería; it does not invent an exact parking-lot coordinate. Check the map with the host before sharing.

## Local development
Install Go (version from `backend/go.mod`; automatic toolchain download is supported).

PowerShell:
```powershell
$env:ADMIN_KEY = 'local-development-key-at-least-32-characters'
cd backend
go run ./cmd/server
```
Bash:
```bash
cd backend
ADMIN_KEY=local-development-key-at-least-32-characters go run ./cmd/server
```
Open `http://localhost:8080/?i=0123456789abcdef0123456789abcdef`.
The local database is in memory and resets on restart. The admin key above is only an example for localhost. To use DynamoDB locally, set `TABLE_NAME`, `AWS_PROFILE` and `AWS_REGION`.

## Deploy from Windows or Linux
Prerequisites: Python 3.10+, Go, Node.js 22+, AWS CLI v2, Pulumi CLI. Install from their official sites. You do **not** need a Pulumi account.

1. Configure AWS SSO with `aws configure sso` and select a role permitted to create the stack resources.
2. Add the domain to Cloudflare and activate its nameservers. Obtain the Zone ID and a token scoped to that zone with **DNS Edit** and **Zone Read**.
3. Run `python scripts/deploy.py` (Windows may use `py scripts/deploy.py`).
4. The wizard checks tools, logs in with AWS SSO when needed, creates/uses an encrypted versioned private S3 state bucket, asks for a Pulumi encryption passphrase, and requests the Cloudflare token without echoing it.
5. It generates encrypted admin/import secrets, compiles the Lambda, checks TypeScript, and runs `pulumi preview`. Type `DESPLEGAR` to apply. State bootstrap has its own `CREAR` prompt.
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
cd backend
go run ./cmd/import-guests -file ../my-guests.csv          # validate only
go run ./cmd/import-guests -file ../my-guests.csv -apply   # create missing families
```
Existing records are never overwritten; RSVPs, sending and assignments are preserved. Repeating an import with the same namespace/phone is safe. Changing a phone changes derived ID unless an explicit original `id` is retained. The CLI does not update existing contacts; correct/import before sending or use a reviewed database maintenance operation.

## Connect Android
In `/send.html`, create a device credential. Copy the server URL and credential into the app. Assign families before sending; assignments are shown in both reports. Revoking a device blocks new API calls but does not undo SMS already dispatched. Keep the app open for the short paced batch. Confirm real-phone permissions/SIM/carrier behavior with one recipient before the remaining 49.

## Verification
```bash
cd backend
go test -race ./...
go vet ./...
cd ../infra
npm ci
npm run check
cd ..
python -m unittest discover -s scripts -p 'test_*.py'
python scripts/build.py
```
CI builds an executable Linux arm64 `bootstrap` ZIP. AWS deployment, carrier delivery and physical Android permissions require your environment. See [API contract](docs/api.md), [design](docs/superpowers/specs/2026-10-19-invitation.md), and [plan](docs/superpowers/plans/2026-10-19-invitation.md).

For the optional browser smoke check, start the local server, install Playwright and Chromium (`npm install --no-save playwright && npx playwright install chromium`), and run `ADMIN_KEY=your-local-key node scripts/browser-smoke.cjs`. It exercises RSVP, admin login, device creation and mobile layout; screenshots go to `build/screenshots`.
