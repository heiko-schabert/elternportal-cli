package portal

import (
	"context"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Notice struct {
	Title    string `json:"title"`
	Period   string `json:"period,omitempty"`
	Text     string `json:"text"`
	Archived bool   `json:"archived,omitempty"`
}

type Bulletin struct {
	Notices []Notice `json:"notices"`
}

func (c *Client) Bulletin(ctx context.Context) (Bulletin, error) {
	s, err := c.page(ctx, "/aktuelles/schwarzes_brett")
	if err != nil {
		return Bulletin{}, err
	}
	return parseBulletin(s), nil
}

func parseBulletin(s *goquery.Selection) Bulletin {
	out := Bulletin{Notices: []Notice{}}
	s.Find("div.card").Each(func(_ int, card *goquery.Selection) {
		title := strings.TrimSpace(card.Find("h4, h3, strong").First().Text())
		body := card.Find(".card-body").First()
		if body.Length() == 0 {
			body = card
		}
		t := strings.TrimSpace(strings.TrimPrefix(text(body), title))
		out.Notices = append(out.Notices, Notice{Title: title, Text: t})
	})
	// Expired notices move to a collapsed archive as wells: first row date
	// range and title, second row body.
	s.Find("div.well").Each(func(_ int, well *goquery.Selection) {
		rows := well.ChildrenFiltered("div.row")
		out.Notices = append(out.Notices, Notice{
			Title:    norm(rows.Eq(0).Find("h4").Text()),
			Period:   norm(rows.Eq(0).Find("p").First().Text()),
			Text:     text(rows.Eq(1)),
			Archived: well.ParentsFiltered(".arch").Length() > 0,
		})
	})
	return out
}
