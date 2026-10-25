# Architecture — foundation milestone

- Go owns application logic, validation, tasks, and RouterOS access.
- The HTTP API exposes narrowly scoped JSON endpoints; the UI never sends raw RouterOS commands.
- React + TypeScript + Bootstrap RTL is shared by local web and the future desktop shell.
- SQLite is local and owned by one application process. POS clients must not open its file directly.
- The RouterOS adapter is the only package depending on `routeros-api-Go`.
- HTTP binds to loopback only. LAN remains disabled until authentication, HTTPS, origin/CSRF protection, and deployment hardening are implemented.
- Currency is a single three-letter code selected during setup and locked after the first persisted financial transaction.

## Foundation endpoints
- `GET /api/health`
- `GET /api/settings`
- `PUT /api/settings`
- `POST /api/router/test` — transient credentials; reads system information only.

HotSpot, User Manager, scratch-card designer, POS, licensing, multi-tenancy and LAN mode are later milestones.
