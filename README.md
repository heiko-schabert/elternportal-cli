# elternportal-mcp

MCP-Server für das [Eltern-Portal](https://www.eltern-portal.org) (art soft and more). Stellt Elternbriefe, Termine, Schwarzes Brett, Vertretungsplan und die Kommunikation mit Lehrkräften als Tools für KI-Assistenten bereit.

## Installation

```bash
go install .
```

Für PDF-Anhänge wird `pdftotext` (poppler) im `PATH` benötigt. Fehlt es, liefern die Tools die Metadaten und einen Hinweis statt des PDF-Texts.

## Konfiguration

Env-Variablen haben Vorrang vor `~/.mcp-server-config/elternportal_mcp/.env` (gleiche Datei wie beim Python-Paket `elternportal-mcp`).

| Key | Pflicht | Bedeutung |
|---|---|---|
| `ELTERNPORTAL_URL` | ja | z.B. `https://schule.eltern-portal.org` |
| `ELTERNPORTAL_USER` | ja | Login-E-Mail |
| `ELTERNPORTAL_PASSWORD` | ja | Passwort |
| `ELTERNPORTAL_ALLOW_WRITE` | nein | `1` aktiviert schreibende Tools |

```bash
claude mcp add -s user elternportal ~/go/bin/elternportal-mcp
```

## Tools

Lesen:

- `check_login`, `list_kinder`
- `get_schulaufgaben`, `get_termine`, `get_schwarzes_brett`, `get_vertretungsplan`
- `list_elternbriefe`, `get_elternbrief`
- `list_nachrichten`, `get_nachricht`, `list_lehrkraefte`

Schreiben (nur mit `ELTERNPORTAL_ALLOW_WRITE=1`):

- `elternbrief_bestaetigen`
- `send_nachricht` (Antwort im Thread), `neue_nachricht`, `klassenleitung_anfrage`

Bei mehreren Kindern nehmen kindbezogene Tools einen Parameter `kind` (Vorname). `get_nachricht` öffnet ungelesene Threads nur mit `ungelesen_oeffnen=true`, weil das Portal sie beim Öffnen als gelesen markiert.

Module, die eine Schule deaktiviert hat, melden „Modul an dieser Schule nicht aktiv".
