# Cloud component (planned)

This directory reserves the independent Cloudflare Worker/D1 and website
component. It currently contains no service, dependencies, or deployment.

When implemented, keep the API source, website assets, `wrangler.jsonc`, package
manifest/lockfile, and SQL migrations here. Its development and deployment
commands must work without building or running the Windows desktop app.

See [development across operating systems](../DEVELOPMENT.md) for the repository
workflow, credentials guidance, and agreed initial sync scope.

See [planned capture and sync](../docs/SYNC.md) for the Worker/D1 inbox, optional
15-minute desktop polling, manual sync, delivery guarantees and deferred clients.
No Cloudflare login or Node installation is required just to build the existing
Windows desktop app.
