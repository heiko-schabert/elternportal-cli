package main

import (
	"context"
	"elternportal-cli/portal"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
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
			res, err := callTool(cmd.Context(), cs, t.Name, toolArgs(cmd.Flags(), s))
			if err != nil {
				return err
			}
			return printJSON(cmd.OutOrStdout(), res.StructuredContent)
		},
	}
	addFlags(cmd, s)
	return cmd, nil
}

// run builds the command tree from the tools of an in-process MCP session and
// executes args. ready reports missing credentials only when a command needs them.
func run(ctx context.Context, srv *mcp.Server, c *portal.Client, ready func() error, args []string, stdout, stderr io.Writer) int {
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
		Short: "Eltern-Portal from the terminal; JSON output",
		Long: `Eltern-Portal from the terminal; JSON output.

Credentials: ELTERNPORTAL_URL, ELTERNPORTAL_USER, ELTERNPORTAL_PASSWORD
as environment variables or in ~/.config/elternportal/env.
Write commands require ELTERNPORTAL_ALLOW_WRITE=1.`,
		Example:       "  elternportal-cli letter --number 49 | jq -r .content",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// Logs go to stderr so stdout stays clean JSON or MCP protocol.
	level := os.Getenv("ELTERNPORTAL_LOG_LEVEL")
	if level == "" {
		level = "warn"
	}
	root.PersistentFlags().StringVar(&level, "log-level", level, "debug, info, warn or error (env ELTERNPORTAL_LOG_LEVEL)")
	root.PersistentPreRunE = func(*cobra.Command, []string) error {
		var l slog.Level
		if err := l.UnmarshalText([]byte(level)); err != nil {
			return fmt.Errorf("--log-level: %w", err)
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: l})))
		return nil
	}
	for _, t := range tools.Tools {
		cmd, err := toolCommand(cs, t, ready)
		if err != nil {
			return fail(err)
		}
		root.AddCommand(cmd)
	}
	var addr string
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server for AI assistants (stdio or HTTP)",
		Long: `MCP server for AI assistants. Defaults to stdio; with --http it serves
Streamable HTTP. HTTP mode has no authentication of its own: bind it to a
private interface only, e.g. the Tailscale IP.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ready(); err != nil {
				return err
			}
			if addr == "" {
				return srv.Run(cmd.Context(), &mcp.StdioTransport{})
			}
			slog.InfoContext(cmd.Context(), "serving MCP over HTTP", "addr", addr)
			return http.ListenAndServe(addr, httpHandler(srv))
		},
	}
	mcpCmd.Flags().StringVar(&addr, "http", "", "address for Streamable HTTP, e.g. 100.64.0.1:8080")
	root.AddCommand(mcpCmd)
	root.AddCommand(downloadCommand(cs, ready), syncCommand(c, ready))
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		return fail(err)
	}
	return 0
}

func httpHandler(srv *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}

// callTool turns a tool error result into a Go error.
func callTool(ctx context.Context, cs *mcp.ClientSession, name string, args map[string]any) (*mcp.CallToolResult, error) {
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return nil, err
	}
	if res.IsError {
		var msgs []string
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				msgs = append(msgs, tc.Text)
			}
		}
		return nil, errors.New(strings.Join(msgs, "\n"))
	}
	return res, nil
}

func printJSON(w io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(w, string(b))
	return nil
}

// downloadCommand saves the files that get_letter/get_message embed, so the
// terminal uses the same tools as MCP clients.
func downloadCommand(cs *mcp.ClientSession, ready func() error) *cobra.Command {
	var letter, thread int
	var openUnread bool
	var child, dir string
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Save the original files of a letter or message thread",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ready(); err != nil {
				return err
			}
			args := map[string]any{"include_files": true}
			if child != "" {
				args["child"] = child
			}
			var name string
			switch {
			case letter > 0 && thread == 0:
				name, args["number"] = "get_letter", letter
			case thread > 0 && letter == 0:
				name, args["thread_id"], args["open_unread"] = "get_message", thread, openUnread
			default:
				return errors.New("pass exactly one of --letter or --thread-id")
			}
			res, err := callTool(cmd.Context(), cs, name, args)
			if err != nil {
				return err
			}
			saved := []string{}
			for _, c := range res.Content {
				r, ok := c.(*mcp.EmbeddedResource)
				if !ok || r.Resource == nil {
					continue
				}
				n, err := url.PathUnescape(path.Base(r.Resource.URI))
				if err != nil {
					return err
				}
				p, err := portal.SaveFile(dir, portal.File{Name: n, Data: r.Resource.Blob})
				if err != nil {
					return err
				}
				slog.InfoContext(cmd.Context(), "saved file", "path", p)
				saved = append(saved, p)
			}
			return printJSON(cmd.OutOrStdout(), map[string][]string{"files": saved})
		},
	}
	cmd.Flags().IntVar(&letter, "letter", 0, "letter number")
	cmd.Flags().IntVar(&thread, "thread-id", 0, "thread_id from the message list")
	cmd.Flags().BoolVar(&openUnread, "open-unread", false, "also open an unread thread (the portal marks it read)")
	cmd.Flags().StringVar(&child, "child", "", "child's first name; only needed with several children")
	cmd.Flags().StringVar(&dir, "dir", ".", "target directory")
	return cmd
}

// syncCommand uses the client directly: skipping existing files needs the
// file names before downloading, which the tools do not expose.
func syncCommand(c *portal.Client, ready func() error) *cobra.Command {
	var child, dir string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Save all letter and message files missing in a directory; unread threads are skipped",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := ready(); err != nil {
				return err
			}
			release, err := c.UseChild(cmd.Context(), child)
			if err != nil {
				return err
			}
			defer release()
			res, err := c.Sync(cmd.Context(), dir)
			if err != nil {
				return err
			}
			return printJSON(cmd.OutOrStdout(), res)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "archive directory")
	cmd.MarkFlagRequired("dir")
	cmd.Flags().StringVar(&child, "child", "", "child's first name; only needed with several children")
	return cmd
}
