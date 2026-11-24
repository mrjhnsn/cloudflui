# cloudflui

A terminal UI for managing Cloudflare DNS zones, records, and Workers deployments.

The UI is built with [OpenTUI](https://opentui.com) (Rust). It drives a small Go
backend that owns the Cloudflare SDK and config store, speaking newline-delimited
JSON-RPC over stdin/stdout.

## Features

- **Zones** — list all zones with status indicators, select one to manage
- **Records** — list, create, edit, delete, and toggle proxy on DNS records (A, AAAA, CNAME, MX, TXT)
- **Editor** — inline form for creating and editing records
- **Config** — manage multiple API token profiles stored in TOML
- **Workers** — list Workers scripts and their deployment history

## Architecture

```
┌──────────────────────┐   JSON-RPC (stdin/stdout)   ┌──────────────────────┐
│  ui/  (Rust, OpenTUI) │ ──────────────────────────▶ │  cloudflui (Go)     │
│  renders the five     │ ◀────────────────────────── │  Cloudflare SDK     │
│  panes, key handling  │    newline-delimited JSON    │  config store (TOML)│
└──────────────────────┘                              └──────────────────────┘
```

The frontend spawns the Go backend as a subprocess and treats it as the single
source of truth for data. The backend exposes one method per operation
(`list_zones`, `list_records`, `create_record`, `save_profile`, …) and the
frontend never talks to Cloudflare directly.

## Build

The Go backend:

```sh
go build -o cloudflui .
```

The Rust UI (needs Rust 1.88+):

```sh
cd ui
scripts/fetch-opentui.sh   # downloads the OpenTUI native library
cargo build --release
```

The UI resolves the backend via the `CLOUDFLUI_BACKEND` env var, falling back to
`cloudflui` on `PATH`. Install both on `PATH`, or point the UI at a specific
backend binary:

```sh
CLOUDFLUI_BACKEND=./cloudflui ./ui/target/release/cloudflui-ui
```

## Configuration

Profiles are stored in `~/.config/cloudflui/config.toml`:

```toml
default_profile = "main"

[[profiles]]
name = "main"
api_key = "your-api-token"
account_id = "your-account-id"
```

You can also create profiles from inside the app (Config pane, tab 4).

Get an API token from the Cloudflare dashboard: **My Profile → API Tokens → Create Token**. The token needs `Zone:Read`, `Zone:DNS:Edit`, and `Workers Scripts:Read` permissions.

## Usage

```
tab / 1-5   switch panes
j / k       move selection
pgup / pgdn page up / down
g / G       jump to top / bottom
enter       select / edit field
n           new record / new profile
p           toggle proxy
d           delete record / profile
s           save form
a           activate profile (current session)
m           set default profile (persisted)
r           refresh
esc         back
q / ctrl+c  quit
```

## Profiles

The **active profile** is the one used for API calls in the current session. The **default profile** is the one used on startup.

- `a` on a profile switches the active profile immediately (not persisted)
- `m` on a profile makes it the default and activates it (persisted to `config.toml`)

## JSON-RPC protocol

The backend reads one JSON request per line on stdin and writes one response per
line on stdout. Each request has an `id`, `method`, and optional `params`; each
response has the matching `id` plus a `result` or an `error`.

| method               | params                                  | result          |
| -------------------- | --------------------------------------- | --------------- |
| `ping`               | —                                       | `{ ok: true }`  |
| `get_config`         | —                                       | `Config`        |
| `list_zones`         | —                                       | `[Zone]`        |
| `list_records`       | `{ zone_id }`                           | `[Record]`      |
| `create_record`      | `{ zone_id, name, type, content, ttl, proxied, comment }` | `{}` |
| `update_record`      | `{ zone_id, record_id, name, type, content, ttl, proxied, comment }` | `{}` |
| `delete_record`      | `{ zone_id, record_id }`                | `{}`            |
| `list_workers`       | `{ account_id }`                        | `[WorkerScript]`|
| `list_deployments`   | `{ account_id, script }`                | `[Deployment]`  |
| `save_profile`       | `{ name, api_key, account_id }`         | `Config`        |
| `delete_profile`     | `{ name }`                              | `Config`        |
| `activate_profile`   | `{ name }`                              | `Config`        |
| `set_default_profile`| `{ name }`                              | `Config`        |

## Project structure

```
main.go                     Go entry point: the JSON-RPC backend
internal/backend/           RPC server + backend operations
internal/api/               Cloudflare SDK wrappers
internal/config/            TOML profile store
ui/                         Rust OpenTUI frontend
ui/src/main.rs              panes, key handling, state
ui/src/backend.rs           backend subprocess client
ui/scripts/fetch-opentui.sh downloads the OpenTUI native library
```

## Notes

Zone and record listings follow Cloudflare's pagination, so the full set is
loaded rather than the first page.
