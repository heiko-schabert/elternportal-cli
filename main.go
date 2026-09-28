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
	Nummer int    `json:"nummer,omitempty" jsonschema:"Nummer des Elternbriefs, z.B. 134"`
	Titel  string `json:"titel,omitempty" jsonschema:"Teil des Titels, Groß/Klein egal"`
}

type loginStatus struct {
	Status string `json:"status"`
}

// tool hides the SDK's result plumbing; the SDK renders Out as structured
// content plus JSON text, and a returned error as an IsError result.
func tool[In, Out any](s *mcp.Server, name, desc string, fn func(context.Context, In) (Out, error)) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
			out, err := fn(ctx, in)
			return nil, out, err
		})
}

func newServer(c *portal.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "elternportal", Version: "v0.1.0"}, nil)
	tool(s, "check_login", "Prüft, ob Login ins Eltern-Portal funktioniert.",
		func(ctx context.Context, _ none) (loginStatus, error) {
			if err := c.CheckLogin(ctx); err != nil {
				return loginStatus{}, err
			}
			return loginStatus{Status: "ok"}, nil
		})
	tool(s, "get_schulaufgaben", "Schulaufgaben- und Prüfungstermine der Klasse (Datum, Beschreibung).",
		func(ctx context.Context, _ none) (portal.Termine, error) { return c.Schulaufgaben(ctx) })
	tool(s, "get_termine", "Allgemeine Schultermine (Ferien, Veranstaltungen): Datum, Zeit, Beschreibung.",
		func(ctx context.Context, _ none) (portal.Termine, error) { return c.Termine(ctx) })
	tool(s, "get_schwarzes_brett", "Aushänge vom Schwarzen Brett (Titel, Zeitraum, Text; archiv=true für abgelaufene).",
		func(ctx context.Context, _ none) (portal.SchwarzesBrett, error) { return c.SchwarzesBrett(ctx) })
	tool(s, "get_vertretungsplan", "Vertretungsplan: Stand und Tage mit Vertretungen (Stunde, betroffene Lehrkraft, Vertretung, entfallenes Fach, Fach, Raum, Info).",
		func(ctx context.Context, _ none) (portal.Vertretungsplan, error) { return c.Vertretungsplan(ctx) })
	tool(s, "list_elternbriefe", "Elternbriefe mit Nummer, Titel, Datum, Klassen, Bestätigungsstatus und ob eine Datei anhängt.",
		func(ctx context.Context, _ none) (portal.Elternbriefe, error) { return c.Elternbriefe(ctx) })
	tool(s, "get_elternbrief", "Inhalt eines Elternbriefs als Text. nummer (exakt) oder titel (Teilstring, neuester Treffer) angeben.",
		func(ctx context.Context, in briefArgs) (portal.ElternbriefInhalt, error) {
			return c.Elternbrief(ctx, in.Nummer, in.Titel)
		})
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
