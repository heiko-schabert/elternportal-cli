package portal

import (
	"context"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Termin struct {
	Datum        string `json:"datum"`
	Beschreibung string `json:"beschreibung"`
}

type Schulaufgaben struct {
	Klasse  string   `json:"klasse"`
	Termine []Termin `json:"termine"`
}

var (
	klasseRe = regexp.MustCompile(`\((\w+)\)`)
	datumRe  = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}`)
)

func (c *Client) Schulaufgaben(ctx context.Context) (Schulaufgaben, error) {
	s, err := c.page(ctx, "/service/termine/liste/schulaufgaben")
	if err != nil {
		return Schulaufgaben{}, err
	}
	return parseSchulaufgaben(s), nil
}

func parseSchulaufgaben(s *goquery.Selection) Schulaufgaben {
	out := Schulaufgaben{Termine: []Termin{}}
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
		datum := strings.TrimSpace(td.Eq(0).Text())
		desc := strings.TrimSpace(td.Eq(2).Text())
		if datumRe.MatchString(datum) && desc != "" {
			out.Termine = append(out.Termine, Termin{Datum: datum, Beschreibung: desc})
		}
	})
	return out
}
