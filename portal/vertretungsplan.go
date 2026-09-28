package portal

import (
	"context"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Vertretung struct {
	Stunde     string `json:"stunde"`
	Betrifft   string `json:"betrifft"`
	Vertretung string `json:"vertretung"`
	FachAlt    string `json:"fach_alt,omitempty"`
	Fach       string `json:"fach"`
	Raum       string `json:"raum"`
	Info       string `json:"info"`
}

type Tag struct {
	Datum        string       `json:"datum"`
	KW           string       `json:"kw"`
	Vertretungen []Vertretung `json:"vertretungen"`
}

type Vertretungsplan struct {
	Stand string `json:"stand"`
	Tage  []Tag  `json:"tage"`
}

var standRe = regexp.MustCompile(`Stand:\s*(.+)`)

func (c *Client) Vertretungsplan(ctx context.Context) (Vertretungsplan, error) {
	s, err := c.page(ctx, "/service/vertretungsplan")
	if err != nil {
		return Vertretungsplan{}, err
	}
	return parseVertretungsplan(s), nil
}

// norm collapses whitespace including &nbsp;, which the portal pads cells with.
func norm(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// parseVertretungsplan walks day headers and tables in document order; each
// table belongs to the preceding "Datum - KW" header.
func parseVertretungsplan(s *goquery.Selection) Vertretungsplan {
	vp := Vertretungsplan{Tage: []Tag{}}
	s.Find("div.list, table").Each(func(_ int, el *goquery.Selection) {
		if goquery.NodeName(el) != "table" {
			t := norm(el.Text())
			if m := standRe.FindStringSubmatch(t); m != nil {
				vp.Stand = m[1]
			} else if el.HasClass("bold") {
				datum, kw, _ := strings.Cut(t, " - ")
				vp.Tage = append(vp.Tage, Tag{Datum: datum, KW: kw, Vertretungen: []Vertretung{}})
			}
			return
		}
		if len(vp.Tage) == 0 {
			return
		}
		tag := &vp.Tage[len(vp.Tage)-1]
		el.Find("tr").Each(func(_ int, tr *goquery.Selection) {
			td := tr.Find("td")
			if td.Length() < 6 || td.HasClass("table_header") {
				return
			}
			// A struck-through subject is the cancelled one; the rest is the replacement.
			fach := td.Eq(3)
			alt := fach.Find(`span[style*="line-through"]`).Text()
			tag.Vertretungen = append(tag.Vertretungen, Vertretung{
				Stunde:     norm(td.Eq(0).Text()),
				Betrifft:   norm(td.Eq(1).Text()),
				Vertretung: norm(td.Eq(2).Text()),
				FachAlt:    norm(alt),
				Fach:       norm(strings.Replace(fach.Text(), alt, "", 1)),
				Raum:       norm(td.Eq(4).Text()),
				Info:       norm(td.Eq(5).Text()),
			})
		})
	})
	return vp
}
