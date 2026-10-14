# Pinny capture Worker

Independent TypeScript Worker, mobile-friendly capture form and D1 inbox. Requires
Node 24 and npm; no desktop SDK or Cloudflare login is needed for local work.
Wrangler is a **local dependency**; `wrangler.jsonc` is the configuration source
of truth. Use WSL for npm, Wrangler and Git in this checkout.

## Current deployment

The capture site is deployed at https://pinny-capture.leog91.workers.dev.
`wrangler.jsonc` contains the deployed D1 ID; `--local` still uses separate data.

For this installation, generated app credentials are kept in ignored
`cloud/.env.production` and in Cloudflare secrets. The website uses its
`CAPTURE_TOKEN`. Import ignored `cloud/sync.local.json` using Pinny's
**Configure sync...**, then choose **Sync now** to connect the desktop.
These local files are plaintext secrets: keep them private, move credentials to
your password manager, and never paste their contents into chat or commit them.
They are not included in a source checkout.

## Local development

```bash
cd cloud
npm ci
cp .dev.vars.example .dev.vars
# Replace both placeholders with separate random values, at least 32 characters.
# Generate each locally, for example: openssl rand -hex 32
npm run db:local
npm run dev
```

Open `http://localhost:8787`. Enter the **capture** app credential in the form.
It is not embedded in the page, sent in a URL or persisted by the browser. A
pending submission's ID/content is saved in localStorage before sending, so a
reload can retry the exact payload. Content stays on that browser until success
or explicit discard; it is not encrypted. Use a private device and HTTPS in
production. Discard does not revoke a submission that reached the server.

The desktop uses the separate **sync** app credential, not the capture credential.
See [API, delivery guarantees and desktop setup](../docs/SYNC.md).

```bash
npm run typecheck
npm test                 # real Worker/D1 tests plus browser submission/retry logic
npm run build           # bundle/dry-run, NOT deployment
npm audit
```

`.wrangler/state/` holds only local D1 data. Tests use their own disposable database.
Local commands explicitly use `--local`; deployed data requires `--remote`.
`.dev.vars`, `.env`, `node_modules` and Wrangler state are ignored. Do not commit
credentials, runtime data or request bodies. The service does not log credentials
or notes; Worker observability is disabled in configuration. Do not enable body
logging in additional tooling.

## Deployment (free plan only, after local checks)

From WSL, with your approval and a Cloudflare account:

```bash
npx wrangler login           # browser OAuth; never paste account tokens into chat
npx wrangler whoami
npx wrangler d1 create pinny-inbox
```

Replace the placeholder `database_id` in `wrangler.jsonc` with the returned UUID
(not a secret). Use an existing database only if dedicated to this inbox. Then:

```bash
npm run db:remote
npx wrangler secret put CAPTURE_TOKEN  # enter a random app credential locally
npx wrangler secret put SYNC_TOKEN     # different random app credential
npm run deploy
```

No command enables a paid plan. Check the account is on Workers Free before
proceeding; stop rather than accept a paid upgrade. Remote resources and secrets
are never required by local tests or CI. Production and local credentials should
be different. Only app credentials are given to clients; Cloudflare deployment
credentials must never be shipped to a client. Rotate app credentials with
`wrangler secret put` and update the relevant client. Desktop credential rotation
on the same endpoint does not change receipt identity.

V1 has one private inbox and one desktop consumer, not user accounts or
multi-device fan-out. Treat app credentials as high-entropy passwords. Delivered
records (including content) are retained to reject changed-payload retries.
There is no automatic expiry, rate quota or purge UI; monitor free-plan D1
capacity. At a capacity limit, submissions fail visibly and remain in browser
retry state. Do not purchase more capacity automatically.
