// Command elternportal-mcp serves Eltern-Portal data over MCP stdio.
package main

import (
	"context"
	"elternportal-mcp/portal"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type none struct{}

type briefArgs struct {
	kindArg
	Nummer int    `json:"nummer,omitempty" jsonschema:"Nummer des Elternbriefs, z.B. 134"`
	Titel  string `json:"titel,omitempty" jsonschema:"Teil des Titels, Groß/Klein egal"`
}

type loginStatus struct {
	Status string `json:"status"`
}

type seiteArgs struct {
	kindArg
	Seite int `json:"seite,omitempty" jsonschema:"Seite der Liste, Standard 1"`
}

type threadArgs struct {
	kindArg
	LehrerID         int  `json:"lehrer_id" jsonschema:"lehrer_id aus list_nachrichten"`
	ThreadID         int  `json:"thread_id" jsonschema:"thread_id aus list_nachrichten"`
	UngelesenOeffnen bool `json:"ungelesen_oeffnen,omitempty" jsonschema:"true öffnet auch ungelesene Threads (markiert sie im Portal als gelesen)"`
}

type kindArg struct {
	Kind string `json:"kind,omitempty" jsonschema:"Vorname des Kindes; nur bei mehreren Kindern nötig"`
}

func (k kindArg) kindName() string { return k.Kind }

// tool hides the SDK's result plumbing and, for inputs embedding kindArg,
// selects the child for the duration of the call.
func tool[In, Out any](s *mcp.Server, c *portal.Client, name, desc string, fn func(context.Context, In) (Out, error)) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
			if k, ok := any(in).(interface{ kindName() string }); ok {
				release, err := c.UseKind(ctx, k.kindName())
				if err != nil {
					var zero Out
					return nil, zero, err
				}
				defer release()
			}
			out, err := fn(ctx, in)
			return nil, out, err
		})
}

func newServer(c *portal.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "elternportal", Version: "v0.1.0"}, nil)
	tool(s, c, "check_login", "Prüft, ob Login ins Eltern-Portal funktioniert.",
		func(ctx context.Context, _ none) (loginStatus, error) {
			if err := c.CheckLogin(ctx); err != nil {
				return loginStatus{}, err
			}
			return loginStatus{Status: "ok"}, nil
		})
	tool(s, c, "get_schulaufgaben", "Schulaufgaben- und Prüfungstermine der Klasse (Datum, Beschreibung).",
		func(ctx context.Context, _ kindArg) (portal.Termine, error) { return c.Schulaufgaben(ctx) })
	tool(s, c, "get_termine", "Allgemeine Schultermine (Ferien, Veranstaltungen): Datum, Zeit, Beschreibung.",
		func(ctx context.Context, _ none) (portal.Termine, error) { return c.Termine(ctx) })
	tool(s, c, "get_schwarzes_brett", "Aushänge vom Schwarzen Brett (Titel, Zeitraum, Text; archiv=true für abgelaufene).",
		func(ctx context.Context, _ none) (portal.SchwarzesBrett, error) { return c.SchwarzesBrett(ctx) })
	tool(s, c, "get_vertretungsplan", "Vertretungsplan: Stand und Tage mit Vertretungen (Stunde, betroffene Lehrkraft, Vertretung, entfallenes Fach, Fach, Raum, Info).",
		func(ctx context.Context, _ kindArg) (portal.Vertretungsplan, error) { return c.Vertretungsplan(ctx) })
	tool(s, c, "list_elternbriefe", "Elternbriefe mit Nummer, Titel, Datum, Klassen, Bestätigungsstatus und ob eine Datei anhängt.",
		func(ctx context.Context, _ kindArg) (portal.Elternbriefe, error) { return c.Elternbriefe(ctx) })
	tool(s, c, "get_elternbrief", "Inhalt eines Elternbriefs als Text. nummer (exakt) oder titel (Teilstring, neuester Treffer) angeben.",
		func(ctx context.Context, in briefArgs) (portal.ElternbriefInhalt, error) {
			return c.Elternbrief(ctx, in.Nummer, in.Titel)
		})
	tool(s, c, "list_kinder", "Kinder im Account (ID, Name, Klasse). Namen für den kind-Parameter anderer Tools.",
		func(ctx context.Context, _ none) (portal.Kinder, error) { return c.Kinder(ctx) })
	tool(s, c, "list_nachrichten", "Nachrichten-Threads mit Fachlehrkräften (neueste zuerst, paginiert): Lehrkraft, Betreff, Datum, ungelesen, Anhang.",
		func(ctx context.Context, in seiteArgs) (portal.Nachrichten, error) {
			return c.Nachrichten(ctx, in.Seite)
		})
	tool(s, c, "get_nachricht", "Kompletter Thread mit allen Beiträgen und Anhängen als Text. Ungelesene nur mit ungelesen_oeffnen=true (markiert gelesen) — vorher User fragen.",
		func(ctx context.Context, in threadArgs) (portal.Thread, error) {
			return c.Nachricht(ctx, in.LehrerID, in.ThreadID, in.UngelesenOeffnen)
		})
	tool(s, c, "list_lehrkraefte", "Lehrkräfte, denen man schreiben kann (ID, Name, Funktion).",
		func(ctx context.Context, _ kindArg) (portal.Lehrkraefte, error) { return c.Lehrkraefte(ctx) })
	return s
}

func main() {
	cfg, err := portal.LoadConfig(os.Getenv, portal.DefaultEnvFile())
	if err != nil {
		log.Fatal(err)
	}
	if err := newServer(portal.New(cfg)).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
