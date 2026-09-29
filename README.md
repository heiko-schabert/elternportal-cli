# elternportal-cli

Command line for [Eltern-Portal](https://www.eltern-portal.org) (art soft and more), the parent portal used by many German schools: parent letters, exams, events, bulletin board, substitution plan and teacher messages, readable in the terminal or as JSON. `elternportal-cli mcp` exposes the same commands as an MCP server for AI assistants.

Portal content (letters, messages, notices) stays German; only the tool's interface is English.

## Install

```bash
go install .
```

PDF attachments need `pdftotext` (poppler) on `PATH`; `devenv shell` provides it. Without it, commands return metadata and a note instead of the PDF text.

## Configuration

Environment variables take precedence over `~/.config/elternportal/env` (`$XDG_CONFIG_HOME/elternportal/env`), one `KEY=value` per line.

| Key | Required | Meaning |
|---|---|---|
| `ELTERNPORTAL_URL` | yes | e.g. `https://school.eltern-portal.org` |
| `ELTERNPORTAL_USER` | yes | login email |
| `ELTERNPORTAL_PASSWORD` | yes | password |
| `ELTERNPORTAL_ALLOW_WRITE` | no | `1` enables write commands |
| `ELTERNPORTAL_LOG_LEVEL` | no | `debug`, `info`, `warn` (default) or `error`; same as `--log-level` |

Logs go to stderr, so stdout stays clean JSON (or MCP protocol). `debug` logs every portal request with path, status, size and duration; `info` adds logins, tool calls and saved files. Passwords, cookies and csrf tokens are never logged.

## Usage

```bash
elternportal-cli                                  # command overview
elternportal-cli message -h                       # flags of one command
elternportal-cli letters                          # table
elternportal-cli letter --number 49
elternportal-cli letter --number 49 --json | jq -r .content
elternportal-cli messages --json | jq '.messages[] | select(.unread)'
ELTERNPORTAL_ALLOW_WRITE=1 elternportal-cli reply --thread-id 146807 --text 'Danke!'
elternportal-cli completion fish | source
```

| Command | Portal page |
|---|---|
| `letters`, `letter`, `confirm-letter`, `download --letter` | Elternbriefe |
| `exams` | Schulaufgaben |
| `events` | Allgemeine Termine |
| `bulletin` | Schwarzes Brett |
| `substitutions` | Vertretungsplan |
| `messages`, `message`, `reply`, `new-message`, `teachers`, `download --thread-id` | Kommunikation Eltern/Fachlehrer |
| `contact-class-teacher` | Kommunikation Eltern/Klassenleitung |
| `children` | child selector |

Output is human-readable by default (tables, `Label: value`, texts as paragraphs); `--json` prints the raw result for scripts. Errors go to stderr with exit code 1.

### Original files

```bash
elternportal-cli download --letter 49 --dir ~/Schule          # one letter
elternportal-cli download --thread-id 146807 --dir ~/Schule   # all attachments of a thread
elternportal-cli sync --dir ~/Schule                          # everything missing, e.g. from cron
```

Files are named `letter-<number>-<name>` and `thread-<id>-<name>` and written with mode `0600`. `sync` only downloads files not yet in the directory and skips unread threads (reported as `skipped_unread`), since opening a thread marks it read. Via MCP, `get_letter` and `get_message` take `include_files=true` and return the originals as embedded resources.

With several children, child-specific commands take `--child <first name>`. `message` opens unread threads only with `--open-unread`, because the portal marks them read on open. Modules a school has disabled report "module disabled at this school".

## MCP

```bash
claude mcp add -s user elternportal -- ~/go/bin/elternportal-cli mcp
```

MCP tool names: `check_login`, `list_children`, `get_exams`, `get_events`, `get_bulletin`, `get_substitutions`, `list_letters`, `get_letter`, `list_messages`, `get_message`, `list_teachers`; with write access also `confirm_letter`, `reply`, `new_message`, `contact_class_teacher`.

### Remote (HTTP)

```bash
elternportal-cli mcp --http 100.64.0.1:8080
```

Streamable HTTP without authentication of its own: bind to a private interface only (e.g. the Tailscale IP), never publicly. Client: `claude mcp add -s user -t http elternportal http://<host>:8080/`.

As a systemd service, credentials come from an `EnvironmentFile` (owned `root:root`, mode `0600`; sops-nix/agenix can generate it):

```ini
# /etc/systemd/system/elternportal-cli.service
[Unit]
After=network-online.target tailscaled.service
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/elternportal-cli mcp --http 100.64.0.1:8080
EnvironmentFile=/etc/elternportal-cli.env
Environment=ELTERNPORTAL_LOG_LEVEL=info
DynamicUser=yes
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

```ini
# /etc/elternportal-cli.env
ELTERNPORTAL_URL=https://school.eltern-portal.org
ELTERNPORTAL_USER=parent@example.org
ELTERNPORTAL_PASSWORD='secret$with$dollar'
```

`EnvironmentFile` does not expand `$`; quote values containing spaces or backslashes with single quotes. `pdftotext` must be on the service's `PATH` (NixOS: `path = [ pkgs.poppler-utils ];`). Write commands stay off unless `ELTERNPORTAL_ALLOW_WRITE=1` is set.
