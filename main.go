// Command elternportal-cli reads and writes Eltern-Portal data from the
// terminal and, with the mcp subcommand, serves the same tools over MCP.
package main

import (
	"context"
	"elternportal-cli/portal"
	"encoding/json"
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type none struct{}

type letterArgs struct {
	childArg
	Number       int    `json:"number,omitempty" jsonschema:"letter number, e.g. 134"`
	Title        string `json:"title,omitempty" jsonschema:"part of the title, case-insensitive"`
	IncludeFiles bool   `json:"include_files,omitempty" jsonschema:"also return the original file as an embedded resource (base64, can be large)"`
}

type loginStatus struct {
	Status string `json:"status"`
}

type pageArgs struct {
	childArg
	Page int `json:"page,omitempty" jsonschema:"list page, default 1"`
}

type threadArgs struct {
	childArg
	ThreadID     int  `json:"thread_id" jsonschema:"thread_id from the message list"`
	OpenUnread   bool `json:"open_unread,omitempty" jsonschema:"also open unread threads (the portal marks them read)"`
	IncludeFiles bool `json:"include_files,omitempty" jsonschema:"also return the original files as embedded resources (base64, can be large)"`
}

type numberArgs struct {
	childArg
	Number int `json:"number" jsonschema:"letter number"`
}

type replyArgs struct {
	childArg
	ThreadID int    `json:"thread_id" jsonschema:"thread_id from the message list"`
	Text     string `json:"text" jsonschema:"message text"`
}

type newMessageArgs struct {
	childArg
	TeacherID int    `json:"teacher_id" jsonschema:"id from the teacher list"`
	Subject   string `json:"subject" jsonschema:"subject, max. 128 characters"`
	Text      string `json:"text" jsonschema:"message text"`
}

type contactArgs struct {
	childArg
	RequestType string `json:"type" jsonschema:"one of the portal's German options: Beratungsgespräch (consultation), Bericht über das Notenbild (grade report), Telefontermin (phone call)"`
	Reason      string `json:"reason" jsonschema:"reason for the request"`
}

type sentStatus struct {
	Status string `json:"status"`
}

type childArg struct {
	Child string `json:"child,omitempty" jsonschema:"child's first name; only needed with several children"`
}

func (k childArg) childName() string { return k.Child }

// tool registers a tool whose result is only its output value.
func tool[In, Out any](s *mcp.Server, c *portal.Client, name, desc string, fn func(context.Context, In) (Out, error)) {
	toolFiles(s, c, name, desc, func(ctx context.Context, in In) (Out, []portal.File, error) {
		out, err := fn(ctx, in)
		return out, nil, err
	})
}

// toolFiles hides the SDK's result plumbing, embeds returned files as
// resources and, for inputs embedding childArg, selects the child for the
// duration of the call.
func toolFiles[In, Out any](s *mcp.Server, c *portal.Client, name, desc string, fn func(context.Context, In) (Out, []portal.File, error)) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
			var zero Out
			if k, ok := any(in).(interface{ childName() string }); ok {
				release, err := c.UseChild(ctx, k.childName())
				if err != nil {
					return nil, zero, err
				}
				defer release()
			}
			start := time.Now()
			out, files, err := fn(ctx, in)
			slog.InfoContext(ctx, "tool call", "tool", name, "files", len(files), "duration", time.Since(start).Round(time.Millisecond), "err", err)
			if err != nil {
				return nil, out, err
			}
			// The SDK's own JSON text comes from a map and loses field order,
			// which the CLI's tables rely on; with Content set it adds none.
			b, err := json.Marshal(out)
			if err != nil {
				return nil, zero, err
			}
			res := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
			for _, f := range files {
				res.Content = append(res.Content, &mcp.EmbeddedResource{Resource: &mcp.ResourceContents{
					URI: "elternportal://file/" + url.PathEscape(f.Name), MIMEType: f.ContentType, Blob: f.Data,
				}})
			}
			return res, out, nil
		})
}

func newServer(c *portal.Client, allowWrite bool) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "elternportal", Version: "v0.1.0"}, nil)
	tool(s, c, "check_login", "Checks that logging in to Eltern-Portal works.",
		func(ctx context.Context, _ none) (loginStatus, error) {
			if err := c.CheckLogin(ctx); err != nil {
				return loginStatus{}, err
			}
			return loginStatus{Status: "ok"}, nil
		})
	tool(s, c, "get_exams", "Exam dates of the class (Schulaufgaben): date, description.",
		func(ctx context.Context, _ childArg) (portal.Events, error) { return c.Exams(ctx) })
	tool(s, c, "get_events", "General school events (holidays, events): date, time, description.",
		func(ctx context.Context, _ none) (portal.Events, error) { return c.Events(ctx) })
	tool(s, c, "get_bulletin", "Bulletin board notices (Schwarzes Brett): title, period, text; archived=true for expired ones.",
		func(ctx context.Context, _ none) (portal.Bulletin, error) { return c.Bulletin(ctx) })
	tool(s, c, "get_substitutions", "Substitution plan (Vertretungsplan): last update and days with substitutions (lesson, absent teacher, substitute, cancelled subject, subject, room, info).",
		func(ctx context.Context, _ childArg) (portal.SubstitutionPlan, error) { return c.SubstitutionPlan(ctx) })
	tool(s, c, "list_letters", "Parent letters (Elternbriefe): number, title, date, classes, confirmation status, whether a file is attached.",
		func(ctx context.Context, _ childArg) (portal.Letters, error) { return c.Letters(ctx) })
	toolFiles(s, c, "get_letter", "Content of a parent letter as text. Pass number (exact) or title (substring, newest match).",
		func(ctx context.Context, in letterArgs) (portal.LetterContent, []portal.File, error) {
			return c.Letter(ctx, in.Number, in.Title, in.IncludeFiles)
		})
	tool(s, c, "list_children", "Children on the account (ID, name, class). Names for the child parameter of other tools.",
		func(ctx context.Context, _ none) (portal.Children, error) { return c.Children(ctx) })
	tool(s, c, "list_messages", "Message threads with teachers (newest first, paginated): teacher, subject, date, unread, attachment.",
		func(ctx context.Context, in pageArgs) (portal.Messages, error) {
			return c.Messages(ctx, in.Page)
		})
	toolFiles(s, c, "get_message", "Full thread with all posts and attachments as text. Unread threads only with open_unread=true (marks them read); ask the user first.",
		func(ctx context.Context, in threadArgs) (portal.Thread, []portal.File, error) {
			return c.Message(ctx, in.ThreadID, in.OpenUnread, in.IncludeFiles)
		})
	tool(s, c, "list_teachers", "Teachers you can write to (ID, name, role).",
		func(ctx context.Context, _ childArg) (portal.Teachers, error) { return c.Teachers(ctx) })
	if !allowWrite {
		return s
	}
	tool(s, c, "confirm_letter", "Confirms receipt of a parent letter in the portal (visible to the school). Ask the user first.",
		func(ctx context.Context, in numberArgs) (portal.Letter, error) {
			return c.ConfirmLetter(ctx, in.Number)
		})
	tool(s, c, "reply", "Replies in an existing teacher thread. Sends immediately; agree on the text with the user first.",
		func(ctx context.Context, in replyArgs) (portal.Thread, error) {
			return c.Reply(ctx, in.ThreadID, in.Text)
		})
	tool(s, c, "new_message", "Starts a new conversation with a teacher. Sends immediately; agree on the text with the user first.",
		func(ctx context.Context, in newMessageArgs) (portal.Message, error) {
			return c.NewMessage(ctx, in.TeacherID, in.Subject, in.Text)
		})
	tool(s, c, "contact_class_teacher", "Sends a contact request to the class teacher (Klassenleitung). Sends immediately; agree with the user first.",
		func(ctx context.Context, in contactArgs) (sentStatus, error) {
			if err := c.ContactClassTeacher(ctx, in.RequestType, in.Reason); err != nil {
				return sentStatus{}, err
			}
			return sentStatus{Status: "sent"}, nil
		})
	return s
}

func main() {
	cfg, err := portal.LoadConfig(os.Getenv, portal.DefaultEnvFile())
	ready := func() error { return err }
	c := portal.New(cfg)
	os.Exit(run(context.Background(), newServer(c, cfg.AllowWrite), c, ready, os.Args[1:], os.Stdout, os.Stderr))
}
