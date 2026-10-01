# cloudflui

A terminal UI for managing Cloudflare DNS zones, records, and Workers deployments.

Built with [Bubble Tea v2](https://charm.land/bubbletea), [Lipgloss v2](https://charm.land/lipgloss), and the [cloudflare-go v7](https://github.com/cloudflare/cloudflare-go) SDK.

## Features

- **Zones** — list all zones with status indicators, select one to manage
- **Records** — list, create, edit, delete, and toggle proxy on DNS records (A, AAAA, CNAME, MX, TXT)
- **Editor** — inline form for creating and editing records
- **Config** — manage multiple API token profiles stored in TOML
- **Workers** — list Workers scripts and their deployment history

## Install

```sh
go install .
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

Paste into any text field with your terminal's paste shortcut (e.g. `Cmd+V` or `Ctrl+Shift+V`) while the field is being edited.

## Long lists

The TUI runs in the terminal's alternate screen and fills it completely. The
Zones, Records and Workers lists are windowed to the available height, so
accounts with hundreds of zones or records stay navigable. When a list is
longer than the pane, a footer shows the visible range:

```
41-59 of 312  (page 3/17)
```

`pgup`/`pgdn` move a full page at a time and `g`/`G` jump to the first and
last item. Lists that fit on screen show no footer. Refreshing a list keeps
your position instead of jumping back to the top.

## Profiles

The **active profile** is the one used for API calls in the current session. The **default profile** is the one used on startup.

- `a` on a profile switches the active profile immediately (not persisted)
- `m` on a profile makes it the default and activates it (persisted to `config.toml`)

## Layout

```
cloudflui
[Zones][Records][Editor][Config][Workers]
┌────────────────────────────────────────┐
│ ●  example.com                         │
│ ◌  pending-zone.net                    │
└────────────────────────────────────────┘
Profile: main
tab/1-5: switch  j/k: move  enter: select  ...
```

## Project structure

```
main.go                     entry point
internal/config/            TOML profile store
internal/api/               Cloudflare SDK wrappers
internal/messages/          Bubble Tea messages
internal/styles/            lipgloss theme
internal/components/        the five panes
internal/model/             root model, update, view
```

## Notes

Zone and record listings follow Cloudflare's pagination, so the full set is
loaded rather than the first page.
