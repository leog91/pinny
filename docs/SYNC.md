# Planned capture and sync

Status: design agreed; no Worker, database deployment, website, or desktop sync
implementation exists yet. The current Windows app continues to save local JSON.

## First version

Use a simple mobile-friendly website to submit a new note or todo to an
authenticated Cloudflare Worker API. Persist the item in a D1 inbox. Pinny fetches
pending items with outbound HTTPS requests and saves them locally as new notes.
The PC does not expose an incoming endpoint or maintain a persistent connection.

| Trigger | Planned behavior |
| --- | --- |
| Pinny starts with automatic sync enabled | Fetch pending items immediately |
| Pinny remains running with automatic sync enabled | Check every 15 minutes; skip overlapping requests |
| Automatic sync is disabled in Options | Stop scheduled and startup sync; remember the choice across restarts |
| User selects Sync now | Fetch immediately, including when automatic sync is disabled |
| PC is offline or Pinny is closed | Keep items in D1 until a later successful sync |

Automatic sync should be enabled by default once the user configures a valid
endpoint and app credential. An unconfigured installation makes no sync requests.
The interval is measured while the process runs; no Windows service or cloud cron
job is required. Network requests must not block the Win32 UI thread.

## Delivery and data boundaries

- Give each submission a stable delivery ID and retry key so retries cannot create
  duplicate notes. Persist receipt state across desktop restarts.
- Acknowledge delivery only after the local note has been saved successfully.
  Interrupted requests must leave the cloud item available for retry.
- Fetch bounded batches of pending items or changes using a persisted cursor.
  Advance the cursor only after saving the batch safely. Define the exact API
  contract and recovery behavior during implementation.
- Keep app authentication separate from Cloudflare deployment authentication.
  Protect both submission and fetch endpoints. Do not ship Cloudflare account
  tokens in the website or desktop executable.
- Keep existing local JSON and backup compatibility. Cloud failures must not
  prevent reading, editing, or saving local notes.
- Share content, identity and todo status through the API. Keep desktop window
  geometry, pin state and theme local. Todo presentation and the compatible local
  data representation still need implementation decisions.

The first version is an incoming capture inbox. Sending all existing local notes
to the cloud, editing existing notes remotely, syncing deletions, and merging
simultaneous edits are outside this version. A later two-way protocol needs
revision checks, deletion records and an explicit conflict policy.

## Repository and Cloudflare configuration

Keep the Worker source, basic website, package manifest/lockfile,
`wrangler.jsonc`, and versioned D1 SQL migrations under `cloud/`. Develop and test
that component independently on WSL, Linux, macOS or Windows. The initial website
can be served by the same Worker as the API.

Use Wrangler from the preferred development environment. Keep configuration in
Git, secret values in Cloudflare secrets and ignored local development files,
and local test databases separate from deployed data. The Cloudflare dashboard
is useful for account setup and inspecting usage and logs. MCP is optional.

Start with local API/database tests, then create the remote resources and deploy,
then connect the desktop client. Verify offline delivery, duplicate prevention,
failed local saves, restart recovery and the automatic-sync toggle.

## Later clients

A PWA, mobile app, bots or other desktop clients can use the same documented API.
They own their UI/toolchain and do not depend on Win32 source or Windows build
scripts. PWA and native mobile implementation are deferred; mobile build host
requirements depend on platform and framework.

See [development across operating systems](../DEVELOPMENT.md) for the checkout,
build and verification workflow.
