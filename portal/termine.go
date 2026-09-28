package portal

import (
	"context"
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

type Termin struct {
	Datum        string `json:"datum"`
	Zeit         string `json:"zeit,omitempty"`
	Beschreibung string `json:"beschreibung"`
}

type Termine struct {
	Klasse  string   `json:"klasse,omitempty"`
	Termine []Termin `json:"termine"`
}

var (
	klasseRe = regexp.MustCompile(`\((\w+)\)`)
	datumRe  = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}`)
)

func (c *Client) Schulaufgaben(ctx context.Context) (Termine, error) {
	return c.termine(ctx, "/service/termine/liste/schulaufgaben")
}

func (c *Client) Termine(ctx context.Context) (Termine, error) {
	return c.termine(ctx, "/service/termine/liste/allgemein")
}

func (c *Client) termine(ctx context.Context, path string) (Termine, error) {
	s, err := c.page(ctx, path)
	if err != nil {
		return Termine{}, err
	}
	return parseTermine(s), nil
}

func parseTermine(s *goquery.Selection) Termine {
	out := Termine{Termine: []Termin{}}
	// Class only appears in the active tab label, e.g. "Schulaufgabenplan (6C)".
	if m := klasseRe.FindStringSubmatch(s.Find("a.active").First().Text()); m != nil {
		out.Klasse = m[1]
	}
	// Year/month header rows have one cell; the date column carries the year anyway.
	s.Find("table.termine-table tr").Each(func(_ int, tr *goquery.Selection) {
		td := tr.Find("td")
		if td.Length() < 3 {
			return
		}
		datum := norm(td.Eq(0).Text())
		desc := text(td.Eq(2))
		if datumRe.MatchString(datum) && desc != "" {
			out.Termine = append(out.Termine, Termin{Datum: datum, Zeit: norm(td.Eq(1).Text()), Beschreibung: desc})
		}
	})
	return out
}
