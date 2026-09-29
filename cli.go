package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func isHelp(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

// commandName derives the terminal command from the tool name, so both
// modes share one registry.
func commandName(tool string) string {
	tool = strings.TrimPrefix(strings.TrimPrefix(tool, "get_"), "list_")
	return strings.ReplaceAll(tool, "_", "-")
}

type schema struct {
	Properties map[string]struct {
		Type string `json:"type"`
	} `json:"properties"`
}

// params returns a tool's parameter names in CLI spelling.
func params(inputSchema any) (schema, []string, error) {
	var s schema
	b, err := json.Marshal(inputSchema)
	if err != nil {
		return s, nil, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, nil, err
	}
	var names []string
	for k := range s.Properties {
		names = append(names, strings.ReplaceAll(k, "_", "-"))
	}
	slices.Sort(names)
	return s, names, nil
}

// parseArgs turns key=value pairs into tool arguments, typed by the schema so
// text=49 stays a string while nummer=49 becomes a number.
func parseArgs(args []string, inputSchema any) (map[string]any, error) {
	s, names, err := params(inputSchema)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, a := range args {
		k, v, ok := strings.Cut(a, "=")
		if !ok {
			return nil, fmt.Errorf("%q: key=value erwartet", a)
		}
		key := strings.ReplaceAll(k, "-", "_")
		p, ok := s.Properties[key]
		if !ok {
			return nil, fmt.Errorf("unbekannter Parameter %q; erlaubt: %s", k, strings.Join(names, ", "))
		}
		switch p.Type {
		case "integer":
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("%s: Zahl erwartet, nicht %q", k, v)
			}
			out[key] = n
		case "boolean":
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("%s: true/false erwartet, nicht %q", k, v)
			}
			out[key] = b
		default:
			out[key] = v
		}
	}
	return out, nil
}

// run executes one command against an in-process MCP session and prints the
// tool's JSON result.
func run(ctx context.Context, srv *mcp.Server, args []string, stdout, stderr io.Writer) int {
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
	if len(args) == 0 || isHelp(args[0]) {
		usage(stdout, tools.Tools)
		return 0
	}
	i := slices.IndexFunc(tools.Tools, func(t *mcp.Tool) bool { return commandName(t.Name) == args[0] })
	if i < 0 {
		return fail(fmt.Errorf("unbekannter Befehl %q; ohne Argumente für Hilfe", args[0]))
	}
	tool := tools.Tools[i]
	in, err := parseArgs(args[1:], tool.InputSchema)
	if err != nil {
		return fail(err)
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool.Name, Arguments: in})
	if err != nil {
		return fail(err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if t, ok := c.(*mcp.TextContent); ok {
				fmt.Fprintln(stderr, t.Text)
			}
		}
		return 1
	}
	b, err := json.MarshalIndent(res.StructuredContent, "", "  ")
	if err != nil {
		return fail(err)
	}
	fmt.Fprintln(stdout, string(b))
	return 0
}

func usage(w io.Writer, tools []*mcp.Tool) {
	fmt.Fprintln(w, "Aufruf: elternportal-cli <befehl> [key=value …]\n\nBefehle:")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	tools = slices.Clone(tools)
	slices.SortFunc(tools, func(a, b *mcp.Tool) int { return strings.Compare(commandName(a.Name), commandName(b.Name)) })
	for _, t := range tools {
		_, names, _ := params(t.InputSchema)
		desc, _, _ := strings.Cut(t.Description, ". ")
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", commandName(t.Name), strings.Join(names, " "), desc)
	}
	fmt.Fprintf(tw, "  mcp\t\tMCP-Server über stdio für KI-Assistenten\n")
	tw.Flush()
	fmt.Fprintln(w, "\nBeispiel: elternportal-cli elternbrief nummer=49 | jq -r .inhalt")
	fmt.Fprintln(w, "Schreibbefehle nur mit ELTERNPORTAL_ALLOW_WRITE=1.")
}
