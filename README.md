# elternportal-cli

Kommandozeile für das [Eltern-Portal](https://www.eltern-portal.org) (art soft and more): Elternbriefe, Termine, Schwarzes Brett, Vertretungsplan und Kommunikation mit Lehrkräften, als JSON auf stdout. Mit `elternportal-cli mcp` stellt dasselbe Binary alle Befehle als MCP-Server für KI-Assistenten bereit.

## Installation

```bash
go install .
```

Für PDF-Anhänge wird `pdftotext` (poppler) im `PATH` benötigt; `devenv shell` stellt es bereit. Fehlt es, liefern die Befehle die Metadaten und einen Hinweis statt des PDF-Texts.

## Konfiguration

Env-Variablen haben Vorrang vor `~/.mcp-server-config/elternportal_mcp/.env` (gleiche Datei wie beim Python-Paket `elternportal-mcp`).

| Key | Pflicht | Bedeutung |
|---|---|---|
| `ELTERNPORTAL_URL` | ja | z.B. `https://schule.eltern-portal.org` |
| `ELTERNPORTAL_USER` | ja | Login-E-Mail |
| `ELTERNPORTAL_PASSWORD` | ja | Passwort |
| `ELTERNPORTAL_ALLOW_WRITE` | nein | `1` aktiviert schreibende Befehle |

## Benutzung

```bash
elternportal-cli                                  # Befehlsübersicht
elternportal-cli nachricht -h                     # Flags eines Befehls
elternportal-cli elternbriefe | jq '.briefe[:5]'
elternportal-cli elternbrief --nummer 49 | jq -r .inhalt
elternportal-cli nachrichten | jq '.nachrichten[] | select(.ungelesen)'
ELTERNPORTAL_ALLOW_WRITE=1 elternportal-cli send-nachricht --lehrer-id 29 --thread-id 146807 --text 'Danke!'
elternportal-cli completion fish | source
```

Fehler gehen nach stderr, Exit-Code 1.

Bei mehreren Kindern nehmen kindbezogene Befehle `--kind <Vorname>`. `nachricht` öffnet ungelesene Threads nur mit `--ungelesen-oeffnen`, weil das Portal sie beim Öffnen als gelesen markiert. Module, die eine Schule deaktiviert hat, melden „Modul an dieser Schule nicht aktiv".

## MCP

```bash
claude mcp add -s user elternportal -- ~/go/bin/elternportal-cli mcp
```

Tool-Namen im MCP-Modus: `check_login`, `list_kinder`, `get_schulaufgaben`, `get_termine`, `get_schwarzes_brett`, `get_vertretungsplan`, `list_elternbriefe`, `get_elternbrief`, `list_nachrichten`, `get_nachricht`, `list_lehrkraefte`; mit Schreibrecht zusätzlich `elternbrief_bestaetigen`, `send_nachricht`, `neue_nachricht`, `klassenleitung_anfrage`.

### Remote (HTTP)

```bash
elternportal-cli mcp --http 100.64.0.1:8080
```

Streamable HTTP ohne eigene Authentifizierung: nur an ein privates Interface binden (z.B. Tailscale-IP), nie öffentlich. Client: `claude mcp add -s user -t http elternportal http://<host>:8080/`.

Als systemd-Dienst kommen die Zugangsdaten über `EnvironmentFile` (Datei `root:root`, `0600`; mit sops-nix/agenix erzeugbar):

```ini
# /etc/systemd/system/elternportal-cli.service
[Unit]
After=network-online.target tailscaled.service
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/elternportal-cli mcp --http 100.64.0.1:8080
EnvironmentFile=/etc/elternportal-cli.env
DynamicUser=yes
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

```ini
# /etc/elternportal-cli.env
ELTERNPORTAL_URL=https://schule.eltern-portal.org
ELTERNPORTAL_USER=eltern@example.org
ELTERNPORTAL_PASSWORD='geheim$mit$dollar'
```

`EnvironmentFile` expandiert kein `$`; Werte mit Leerzeichen oder Backslash in einfache Anführungszeichen setzen. `pdftotext` muss im `PATH` des Dienstes liegen (NixOS: `path = [ pkgs.poppler-utils ];`). Schreibbefehle bleiben aus, solange `ELTERNPORTAL_ALLOW_WRITE=1` fehlt.
