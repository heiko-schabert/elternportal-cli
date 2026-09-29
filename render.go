package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"unicode/utf8"
)

// field keeps JSON object keys in document order; a map would sort table
// columns alphabetically.
type field struct {
	key string
	val any
}

type object []field

func decodeOrdered(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		var o object
		for dec.More() {
			k, err := dec.Token()
			if err != nil {
				return nil, err
			}
			v, err := decodeOrdered(dec)
			if err != nil {
				return nil, err
			}
			o = append(o, field{k.(string), v})
		}
		_, err := dec.Token()
		return o, err
	case json.Delim('['):
		a := []any{}
		for dec.More() {
			v, err := decodeOrdered(dec)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		_, err := dec.Token()
		return a, err
	}
	return tok, nil
}

// render prints a tool result for humans: lists of flat objects as tables,
// objects as aligned "Label: value" lines with long texts as paragraphs.
func render(w io.Writer, js []byte) error {
	dec := json.NewDecoder(bytes.NewReader(js))
	dec.UseNumber()
	v, err := decodeOrdered(dec)
	if err != nil {
		return err
	}
	// Wrapper objects like {"letters": [...]} read better as the bare list.
	if o, ok := v.(object); ok && len(o) == 1 {
		if _, isList := o[0].val.([]any); isList {
			v = o[0].val
		}
	}
	var buf bytes.Buffer
	renderValue(&buf, v)
	out := trimLines(buf.String())
	if out == "" {
		out = "(none)\n"
	}
	_, err = io.WriteString(w, out)
	return err
}

func renderValue(buf *bytes.Buffer, v any) {
	switch v := v.(type) {
	case object:
		renderObject(buf, v)
	case []any:
		renderList(buf, v)
	default:
		fmt.Fprintln(buf, scalar(v))
	}
}

func renderObject(buf *bytes.Buffer, o object) {
	tw := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
	var rest []field
	for _, f := range o {
		switch {
		case isEmpty(f.val):
		case isShort(f.val):
			fmt.Fprintf(tw, "%s:\t%s\n", label(f.key), scalar(f.val))
		default:
			rest = append(rest, f)
		}
	}
	tw.Flush()
	for _, f := range rest {
		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		switch v := f.val.(type) {
		case string:
			buf.WriteString(strings.TrimRight(v, "\n") + "\n")
		default:
			fmt.Fprintf(buf, "%s:\n", label(f.key))
			var sub bytes.Buffer
			renderValue(&sub, v)
			buf.WriteString(indent(sub.String()))
		}
	}
}

func renderList(buf *bytes.Buffer, items []any) {
	if len(items) == 0 {
		return
	}
	if cols, ok := tableColumns(items); ok {
		tw := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
		var head []string
		for _, c := range cols {
			head = append(head, strings.ToUpper(label(c)))
		}
		fmt.Fprintln(tw, strings.Join(head, "\t"))
		for _, it := range items {
			var row []string
			for _, c := range cols {
				row = append(row, scalar(lookup(it.(object), c)))
			}
			fmt.Fprintln(tw, strings.Join(row, "\t"))
		}
		tw.Flush()
		return
	}
	for i, it := range items {
		if _, ok := it.(object); ok && i > 0 {
			buf.WriteString("\n")
		}
		renderValue(buf, it)
	}
}

// tableColumns returns the union of keys when every item is a flat object.
func tableColumns(items []any) ([]string, bool) {
	var cols []string
	seen := map[string]bool{}
	for _, it := range items {
		o, ok := it.(object)
		if !ok {
			return nil, false
		}
		for _, f := range o {
			if !isEmpty(f.val) && !isShort(f.val) {
				return nil, false
			}
			if !seen[f.key] {
				seen[f.key] = true
				cols = append(cols, f.key)
			}
		}
	}
	return cols, true
}

func lookup(o object, key string) any {
	for _, f := range o {
		if f.key == key {
			return f.val
		}
	}
	return nil
}

func isEmpty(v any) bool {
	switch v := v.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case object:
		return len(v) == 0
	}
	return false
}

// isShort decides between a table cell or label line and a paragraph.
func isShort(v any) bool {
	switch v := v.(type) {
	case string:
		return !strings.Contains(v, "\n") && utf8.RuneCountInString(v) <= 100
	case object, []any:
		return false
	}
	return true
}

func scalar(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case bool:
		if v {
			return "yes"
		}
		return "no"
	}
	return fmt.Sprint(v)
}

func label(key string) string {
	s := strings.ReplaceAll(key, "_", " ")
	return strings.ToUpper(s[:1]) + s[1:]
}

func indent(s string) string {
	lines := strings.SplitAfter(s, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			lines[i] = "  " + l
		}
	}
	return strings.Join(lines, "")
}

// trimLines drops the padding tabwriter leaves after the last filled column.
func trimLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}
