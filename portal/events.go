package portal

import (
	"context"
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

type Event struct {
	Date        string `json:"date"`
	Time        string `json:"time,omitempty"`
	Description string `json:"description"`
}

type Events struct {
	Class  string  `json:"class,omitempty"`
	Events []Event `json:"events"`
}

var (
	classRe = regexp.MustCompile(`\((\w+)\)`)
	dateRe  = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}`)
)

func (c *Client) Exams(ctx context.Context) (Events, error) {
	return c.events(ctx, "/service/termine/liste/schulaufgaben")
}

func (c *Client) Events(ctx context.Context) (Events, error) {
	return c.events(ctx, "/service/termine/liste/allgemein")
}

func (c *Client) events(ctx context.Context, path string) (Events, error) {
	s, err := c.page(ctx, path)
	if err != nil {
		return Events{}, err
	}
	return parseEvents(s), nil
}

func parseEvents(s *goquery.Selection) Events {
	out := Events{Events: []Event{}}
	// Class only appears in the active tab label, e.g. "Schulaufgabenplan (6C)".
	if m := classRe.FindStringSubmatch(s.Find("a.active").First().Text()); m != nil {
		out.Class = m[1]
	}
	// Year/month header rows have one cell; the date column carries the year anyway.
	s.Find("table.termine-table tr").Each(func(_ int, tr *goquery.Selection) {
		td := tr.Find("td")
		if td.Length() < 3 {
			return
		}
		datum := norm(td.Eq(0).Text())
		desc := text(td.Eq(2))
		if dateRe.MatchString(datum) && desc != "" {
			out.Events = append(out.Events, Event{Date: datum, Time: norm(td.Eq(1).Text()), Description: desc})
		}
	})
	return out
}
