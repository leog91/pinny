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

Open `http://localhost:8787`. Log in with the **capture** app credential (not a
Cloudflare account token). The labeled login form supports password-manager
autofill: save it with the fixed username **Pinny capture** and the capture
credential as its password. Autofill prompts vary by password manager/browser.
Without **Remember this device**, login lasts only for the open page; the raw
credential stays in memory and is cleared on logout/reload. The application never
saves the credential in browser storage or cookies (a password manager may save
it with your permission).

With **Remember this device**, D1 stores only a SHA-256 hash of an independent
256-bit random session token. A persistent `__Host-pinny-session` cookie uses
`Secure; HttpOnly; Path=/; SameSite=Strict`, with no Domain. Sessions expire
**30 days after login**, not extended by use: convenient for a private phone,
with a bounded stolen-device exposure. Login needs only the Worker, not your PC.
The cookie survives closing/reopening the browser or installed app in the same
storage context. Private browsing, browser-data deletion, and other browser/PWA
contexts may require their own login. Production requires HTTPS; localhost is
only a development exception supported by browsers for secure cookies/workers.

**Forget this device / Log out** revokes the current session in D1 and clears its
cookie; it needs connectivity and reports failure rather than pretending to
revoke offline. It does not clear saved note content. Expired/revoked sessions
show the login form again without dropping pending content. Rotating CAPTURE_TOKEN
invalidates all existing device sessions; SYNC_TOKEN remains separate. To revoke
all devices without changing the capture credential, an operator can delete the
`device_sessions` rows in the existing D1 database (requires explicit production
approval). Lost devices should be revoked or the capture credential rotated.
Expired rows are pruned on remembered logins; validation always checks expiry.

### Install and capture offline

Use Android Chrome's **Install app / Add to Home screen** menu (browser wording
varies). The manifest, home-screen icon, standalone display and start URL `/`
open directly to capture. Installation is optional; the normal website works
the same way. Load the site online once to install its service worker. The public
shell is cached explicitly; API requests, authenticated responses, cookies and
credentials are never cached. Offline first-ever visits cannot work.

- **Draft**: editable content auto-saved on input, even offline.
- **Pending**: an immutable ID/payload saved **before** any network submission;
  Send becomes **Retry pending note**. A reload, expired login, lost response or
  server failure preserves that exact payload/ID for duplicate-safe retry.
- **Sending**: awaiting the server; not yet confirmed. Overlapping sends and
  app updates are blocked while the request runs (20-second timeout).
- **Server accepted**: the server confirmed the exact ID. This is **not** proof
  that Windows has imported it. The PC imports separately when sync runs.

On reconnect the page prompts you to tap Retry; login again first if needed.
There is no background sync and no promise of delivery while the app is closed.
Service-worker updates wait for the **Update app** button, save the draft before
reloading and leave pending submissions intact. After success, local saved
content is removed. A storage/quota failure stops sending rather than risking an
unrecoverable ID. Copy visible content somewhere safe if storage is unavailable.
Drafts/pending content use localStorage, unencrypted, until success or explicit
discard. Do not clear browser/site data during recovery. Discard cannot revoke a
note already accepted by the server; a changed payload must use a new ID.

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

### Updating the existing production installation

Do **not** create a new database, replace resources or reset app secrets for this
update. After explicit approval for production migrations/deployment, from
`cloud/` in WSL using the existing Wrangler configuration and console OAuth:

```bash
npx wrangler whoami                 # use existing authentication; login only if needed
npx wrangler d1 migrations list pinny-inbox --remote
npm run db:remote                   # applies 0002_device_sessions.sql; leaves inbox intact
npm run deploy
```

Apply the migration before deploying the Worker. Review the migration list first;
the existing installation should already have `0001_inbox.sql`. Verify HTTPS
login, remembered reload, one capture and Forget after deployment. Existing
bearer clients need no changes; no new secret, binding, paid resource or account
credential is needed. Rollback of the Worker can leave the additive session table
in place; do not roll back/delete inbox data. Changing the public shell later
requires bumping the cache version in `src/pwa.ts`.

### Initial setup only

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
are never required by local tests. Production and local credentials should
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

## Local and Android smoke checks

`npm test` covers real Worker/D1 authentication, session hashing/expiry/revocation,
cookie attributes, Origin/CSRF rejection, bearer/desktop compatibility, and VM
browser tests for draft reload, immutable retries, expired login, storage failure,
send overlap and service-worker cache boundaries/update activation.

On a real Android phone with Chrome, after an approved HTTPS deployment:

1. Open the site, use your password manager to fill the capture password, check
   Remember, log in. Confirm only the capture form is shown. Install from Chrome's
   menu; launch from the home screen, close/reopen and confirm remembered login.
   If the installed context does not share cookies, log in once there too.
2. Type a draft, enable airplane mode, close/reopen the installed app and verify
   the draft remains. Tap Send: it must remain pending, never server-accepted.
   Restore connectivity and tap Retry (log in if prompted). Confirm server
   acceptance and only one inbox item, including after a simulated lost response.
3. Forget online, reload, verify login is required. With a draft/pending note,
   test expiry/revocation using an isolated local/test session, then reauthenticate
   and retry without losing content. Never manipulate production inbox data.
4. Inspect DevTools Application: cookie is HttpOnly/Secure/host-only; localStorage
   holds only draft/pending content; Cache Storage contains only the public shell,
   not `/api/` responses. Check no credential appears in URLs or console logs.
5. With a draft, publish a test shell update on a non-production test deployment.
   Confirm the update waits, cannot reload during Sending, and retains content
   after explicit Update. Test a narrow screen, keyboard-only navigation, and
   the ordinary browser page without installation.

Desktop Chromium checks cannot verify Android installation UI, password-manager
integration, OS storage isolation, or installed-PWA lifecycle; run these device
checks separately.

Local verification (2026-10-17): WSL Node 24 dependency install/audit, type checks,
Worker/D1 and browser-logic tests, local migrations and dry-run build passed.
Desktop Chromium verified remembered login/reload, HttpOnly invisibility to JS,
offline shell reload with draft recovery, pending capture/reconnect/retry, server
acceptance, Forget/reload and a 390px viewport without horizontal overflow. Its
manifest/installability diagnostics reported no errors; cache inspection showed
only the four public shell URLs. No real Android install, OS lifecycle, native
password-manager autofill or Windows desktop runtime check was executed.

Production rollout (2026-10-17, explicitly approved): applied
`0002_device_sessions.sql` to the existing D1 database and deployed Worker version
`846671e5-2267-4ff5-a0dd-5cb06c10a9ef`. The remote migration list is clear. Live
HTTPS checks passed remembered login, cookie attributes, restored authentication,
CSRF rejection, authenticated capture validation, existing capture bearer/desktop
sync credentials, Forget and revoked-session rejection. The temporary test
session was revoked; no test note was added to the production inbox. Chromium
also verified the production manifest/installability diagnostics, offline draft
reload, public-only cache contents and narrow layout. Existing secrets, resources
and billing settings were not replaced or upgraded. Account billing/subscription
inspection was unavailable to the existing OAuth session (403); no paid resource
or plan change was requested.
