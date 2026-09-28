package portal

import (
	"context"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Aushang struct {
	Titel    string `json:"titel"`
	Zeitraum string `json:"zeitraum,omitempty"`
	Text     string `json:"text"`
	Archiv   bool   `json:"archiv,omitempty"`
}

type SchwarzesBrett struct {
	Aushaenge []Aushang `json:"aushaenge"`
}

func (c *Client) SchwarzesBrett(ctx context.Context) (SchwarzesBrett, error) {
	s, err := c.page(ctx, "/aktuelles/schwarzes_brett")
	if err != nil {
		return SchwarzesBrett{}, err
	}
	return parseSchwarzesBrett(s), nil
}

func parseSchwarzesBrett(s *goquery.Selection) SchwarzesBrett {
	out := SchwarzesBrett{Aushaenge: []Aushang{}}
	s.Find("div.card").Each(func(_ int, card *goquery.Selection) {
		titel := strings.TrimSpace(card.Find("h4, h3, strong").First().Text())
		body := card.Find(".card-body").First()
		if body.Length() == 0 {
			body = card
		}
		t := strings.TrimSpace(strings.TrimPrefix(text(body), titel))
		out.Aushaenge = append(out.Aushaenge, Aushang{Titel: titel, Text: t})
	})
	// Expired notices move to a collapsed archive as wells: first row date
	// range and title, second row body.
	s.Find("div.well").Each(func(_ int, well *goquery.Selection) {
		rows := well.ChildrenFiltered("div.row")
		out.Aushaenge = append(out.Aushaenge, Aushang{
			Titel:    norm(rows.Eq(0).Find("h4").Text()),
			Zeitraum: norm(rows.Eq(0).Find("p").First().Text()),
			Text:     text(rows.Eq(1)),
			Archiv:   well.ParentsFiltered(".arch").Length() > 0,
		})
	})
	return out
}
