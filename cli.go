package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// commandName derives the terminal command from the tool name, so both
// modes share one registry.
func commandName(tool string) string {
	tool = strings.TrimPrefix(strings.TrimPrefix(tool, "get_"), "list_")
	return strings.ReplaceAll(tool, "_", "-")
}

func flagName(prop string) string { return strings.ReplaceAll(prop, "_", "-") }

type property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type schema struct {
	Properties map[string]property `json:"properties"`
	Required   []string            `json:"required"`
}

func parseSchema(inputSchema any) (schema, error) {
	var s schema
	b, err := json.Marshal(inputSchema)
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(b, &s)
}

func addFlags(cmd *cobra.Command, s schema) {
	for name, p := range s.Properties {
		f := flagName(name)
		switch p.Type {
		case "integer":
			cmd.Flags().Int(f, 0, p.Description)
		case "boolean":
			cmd.Flags().Bool(f, false, p.Description)
		default:
			cmd.Flags().String(f, "", p.Description)
		}
		if slices.Contains(s.Required, name) {
			cmd.MarkFlagRequired(f)
		}
	}
}

// toolArgs sends only flags the user set, so tools keep their own defaults.
func toolArgs(fs *pflag.FlagSet, s schema) map[string]any {
	in := map[string]any{}
	for name, p := range s.Properties {
		f := flagName(name)
		if !fs.Changed(f) {
			continue
		}
		switch p.Type {
		case "integer":
			in[name], _ = fs.GetInt(f)
		case "boolean":
			in[name], _ = fs.GetBool(f)
		default:
			in[name], _ = fs.GetString(f)
		}
	}
	return in
}

func toolCommand(cs *mcp.ClientSession, t *mcp.Tool, ready func() error) (*cobra.Command, error) {
	s, err := parseSchema(t.InputSchema)
	if err != nil {
		return nil, err
	}
	short, _, _ := strings.Cut(t.Description, ". ")
	cmd := &cobra.Command{
		Use:   commandName(t.Name),
		Short: strings.TrimSuffix(short, "."),
		Long:  t.Description,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ready(); err != nil {
				return err
			}
			res, err := cs.CallTool(cmd.Context(), &mcp.CallToolParams{Name: t.Name, Arguments: toolArgs(cmd.Flags(), s)})
			if err != nil {
				return err
			}
			if res.IsError {
				var msgs []string
				for _, c := range res.Content {
					if tc, ok := c.(*mcp.TextContent); ok {
						msgs = append(msgs, tc.Text)
					}
				}
				return errors.New(strings.Join(msgs, "\n"))
			}
			b, err := json.MarshalIndent(res.StructuredContent, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
	addFlags(cmd, s)
	return cmd, nil
}

// run builds the command tree from the tools of an in-process MCP session and
// executes args. ready reports missing credentials only when a command needs them.
func run(ctx context.Context, srv *mcp.Server, ready func() error, args []string, stdout, stderr io.Writer) int {
	fail := func(err error) int {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ct, st := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		return fail(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "elternportal-cli"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		return fail(err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		return fail(err)
	}
	root := &cobra.Command{
		Use:   "elternportal-cli",
		Short: "Eltern-Portal im Terminal; Ausgabe als JSON",
		Long: `Eltern-Portal im Terminal; Ausgabe als JSON.

Zugangsdaten: ELTERNPORTAL_URL, ELTERNPORTAL_USER, ELTERNPORTAL_PASSWORD
als Env-Variablen oder in ~/.mcp-server-config/elternportal_mcp/.env.
Schreibbefehle nur mit ELTERNPORTAL_ALLOW_WRITE=1.`,
		Example:       "  elternportal-cli elternbrief --nummer 49 | jq -r .inhalt",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	for _, t := range tools.Tools {
		cmd, err := toolCommand(cs, t, ready)
		if err != nil {
			return fail(err)
		}
		root.AddCommand(cmd)
	}
	root.AddCommand(&cobra.Command{
		Use:   "mcp",
		Short: "MCP-Server über stdio für KI-Assistenten",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ready(); err != nil {
				return err
			}
			return srv.Run(cmd.Context(), &mcp.StdioTransport{})
		},
	})
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		return fail(err)
	}
	return 0
}
