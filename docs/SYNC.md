# Capture and incoming sync, version 1

Implemented locally in `cloud/` and `Pinny.Go/`; remote setup is separate. Pinny
continues to use local JSON. A mobile-friendly website submits notes/todos to an
authenticated Worker and D1 inbox. The PC makes outbound requests; there is no
local server, persistent connection, Windows service, cloud cron or PC webhook.

## Desktop setup and triggers

1. Set up the Worker using [cloud/README.md](../cloud/README.md).
2. Copy [sync.example.json](sync.example.json) to a **private `sync.local.json` file**
   (ignored by Git), replacing
   `Endpoint` with the Worker HTTPS origin (no path/query) and `Credential` with
   its `SYNC_TOKEN`. Never use a Cloudflare account token.
3. Choose **⋯ → Configure sync...**, select that JSON file, then **Sync now**.
   Alternatively, while Pinny is closed, write the configuration directly to
   `%LOCALAPPDATA%\Pinny\sync.json`.

Configuration is copied to `%LOCALAPPDATA%\Pinny\sync.json`, separate from a custom
notes folder. The credential is plaintext, protected by the Windows user profile;
do not share this file or a settings-folder backup. Remove it and restart to
disconnect. HTTPS is required except explicit localhost/127.0.0.1/::1 HTTP for
local development. No config or invalid config means **no network requests**.
`--notes` mode never syncs. Environment variables do not configure sync.

| Trigger | Behavior |
| --- | --- |
| Startup, configured and automatic enabled | Fetch immediately |
| While running with automatic enabled | Fetch every 15 minutes; skip overlap |
| **⋯ → Automatic sync**, unchecked | Disable startup and future scheduled requests |
| **⋯ → Sync now** | Fetch even when automatic is disabled |
| Offline, closed or requests/saves fail | Retain items until later successful sync |

Automatic sync defaults on. The checkmark is persisted in the existing
`%LOCALAPPDATA%\Pinny\settings.json` as optional `AutomaticSync`; absent means on.
The folder picker preserves it. Turning it on schedules the next 15-minute check;
use Sync now for an immediate check. Disabling does not cancel an in-flight
delivery. Reconfiguring is blocked during sync. Manual sync reports success or
failure; automatic failures are quiet, without credential/content logging.
A native dialog can defer applying a fetched batch until it closes. HTTP runs
in a Go goroutine with 20-second request timeouts and a five-minute action limit;
Win32 updates and local saves stay on the UI thread. Editing works during requests.

## API contract

Version prefix: `/api/v1`. UTF-8 JSON bodies, `no-store` responses. Both app
secrets must be at least 32 characters; generate separate random values locally.
Bearer API clients use `Authorization: Bearer <app credential>` as before. One
inbox, one desktop consumer. No CORS. Browser requests must be same-origin;
never put credentials in query strings. The website can alternatively capture
using a remembered-device cookie; this never authorizes desktop pending/ack.

### Website device login

`POST /api/v1/session` with the capture bearer credential, JSON
`{"remember":true}` and an exact same-origin `Origin` creates a separate random
256-bit session, hashed in D1. A host-only `__Host-pinny-session` cookie is Secure,
HttpOnly, SameSite=Strict and persistent for a fixed 30 days. Login rotates the
current session. `remember:false` validates the credential, revokes/clears any
current device cookie, and leaves the website using a memory-only bearer login.
The raw capture credential is never placed in browser storage/cookies.

`GET /api/v1/session` returns only `{"authenticated":true|false}` for the cookie.
`DELETE /api/v1/session` revokes the presented session and expires its cookie.
Both session mutations and cookie-authenticated `POST /items` require an exact
same-origin Origin (missing/opaque origins rejected); conflicting cross-site
Fetch Metadata is rejected too. Bearer clients may still omit Origin. No CORS
headers are issued. Session responses are `no-store`, and no token is returned
in a JSON body or URL. Cookie sessions cannot call pending or ack. Capture
credential rotation invalidates all remembered devices; desktop sync is unchanged.
Expiry/revocation preserves browser drafts/pending payloads for reauthentication.
See [installation, offline recovery and device smoke tests](../cloud/README.md).

### POST `/items` — capture credential

```json
{
  "id": "12345678-1234-4234-8234-123456789abc",
  "kind": "todo",
  "text": "Buy milk",
  "title": "Shopping",
  "completed": false
}
```

Create a lowercase UUID once, **before** sending; keep the unchanged payload/ID
until success. `kind` is `note` or `todo`. Text is nonblank, at most 16,000 UTF-8
bytes, with no NUL. Optional title defaults empty, at most 100 Unicode characters,
with no newline/NUL. Optional completed defaults false; notes cannot be completed.
Request bodies are bounded to 70,000 bytes. `200 {"id":"…"}` means stored, including
an exact retry. Same ID with changed content gives `409 {"error":"id_conflict"}`,
even after delivery. The D1 primary key protects concurrent retries.

### GET `/pending` — desktop sync credential

Returns `200 {"items":[...]}`: up to 50 undelivered items, ordered by `createdAt`,
then ID, with submission fields plus UTC `createdAt`. Fetch is non-destructive;
no lease or expiry. Empty means caught up. One action processes up to 20 batches
(1,000 items); remaining items wait for the next action. **No cursor**: only ack
removes items from pending results, so failures cannot advance past unsaved work.

### POST `/ack` — desktop sync credential

```json
{ "ids": ["12345678-1234-4234-8234-123456789abc"] }
```

Accepts 1–50 UUIDs. Returns `200 {"ok":true}`; repeated ack and unknown IDs are
harmless. Acknowledged rows retain IDs **and content** indefinitely for retry
comparison; `delivered_at` excludes them from pending results. They are not
broadcast to other desktops. Accounts, per-consumer delivery and retention/purge
policy are future work.

Errors: `400 invalid_request`, `401 unauthorized`, `403 forbidden`, `404 not_found`,
`409 id_conflict`, `503 temporarily_unavailable`. No error includes note text,
credentials or raw DB exceptions. The desktop rejects redirects rather than
forwarding credentials. `cloud/test/` contains runnable contract examples.

## Local representation and recovery

- A submission creates a new plain-text note. Todos use `[ ] ` or `[x] ` before
  text: deliberately editable text, **not** a new task widget/status model. Local
  checkbox-text edits are not sent to the cloud.
- Local IDs are deterministic UUID-shaped SHA-256-derived values, namespaced by
  Worker origin and submission UUID. Defaults are a visible 320×320 window, Light
  theme, unpinned. Geometry, pin state and theme remain local and absent from API.
- Before importing, atomically write `sync-pending.json`, an ID-only journal in
  the notes folder. Create editors, snapshot all notes including current edits,
  save `notes.json`, then atomically save `sync-receipts.json`. **Only then ack**.
  A failure does not acknowledge or remove the cloud item.
- Receipts prevent recreation even after emptying Trash. Existing note/Trash IDs
  also prevent duplicate import after a lost ack or crash. Startup recovers
  journal IDs only if present in durably saved notes/Trash; unsaved IDs stay for
  cloud retry. Receipt-save failures preserve the journal and must be resolved
  before quitting or trashing incoming notes.
- Local editing remains available after failures. An editor created before a
  failed save can stay visible; retry saves it rather than creating another copy.
  Receipt corruption fails closed, rather than guessing.
- Folder switching copies receipts, notes and Trash, leaving originals. Export
  cannot overwrite active sync files or credentials. Existing arrays, legacy
  single-note files and version-1 backups retain their format. Backups contain
  notes/Trash, **not credentials or the receipt ledger**. For full machine sync
  recovery privately preserve the ledger too. A notes-only restore deduplicates
  IDs still present but cannot remember permanently deleted deliveries.

Keep one process per notes folder; simultaneous JSON writers are unsupported.
Do not remove receipt files while cloud items may be pending. Changing Worker
origin creates a new inbox identity; credential rotation on the same origin
preserves it. Do not repurpose a database's UUIDs for new items.

## Validation and deferred scope

Linux executes portable Go tests (retry/ack/save failure, cancellation, redirects,
validation, identity and todo text), and independent Worker/D1/browser retry-logic
tests. Windows additionally tests real Win32 editors and HTTP: startup, responsive
editing during requests, overlap, failed saves/retry, persisted automatic toggle,
manual path, receipts and folder changes. Cross-compilation alone is **not**
runtime verification. Run checks locally on both platforms; interactive
picker/mobile-layout checks are additional smoke checks.

The capture website is an installable, offline-shell PWA with local drafts and
explicit pending retry, not background delivery or a two-way offline sync engine.
Native mobile apps, bots, persistent connections, PC webhooks, uploading
existing local notes, two-way editing, deletion sync and conflict merging remain
deferred. Future clients can use the API without Win32 code or desktop SDKs.
