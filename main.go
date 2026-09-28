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
		func(ctx context.Context, _ none) (portal.Schulaufgaben, error) { return c.Schulaufgaben(ctx) })
	tool(s, "get_schwarzes_brett", "Aushänge vom Schwarzen Brett (Titel, Text).",
		func(ctx context.Context, _ none) (portal.SchwarzesBrett, error) { return c.SchwarzesBrett(ctx) })
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
