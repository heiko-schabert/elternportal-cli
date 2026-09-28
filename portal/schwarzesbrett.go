package portal

import (
	"context"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Aushang struct {
	Titel string `json:"titel"`
	Text  string `json:"text"`
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
	return out
}
