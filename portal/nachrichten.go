package portal

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const fachlehrerPath = "/meldungen/kommunikation_fachlehrer"

type Nachricht struct {
	LehrerID     int    `json:"lehrer_id"`
	ThreadID     int    `json:"thread_id"`
	Lehrkraft    string `json:"lehrkraft"`
	Betreff      string `json:"betreff"`
	Datum        string `json:"datum"`
	VonLehrkraft bool   `json:"von_lehrkraft"`
	Anhang       bool   `json:"anhang"`
	Ungelesen    bool   `json:"ungelesen"`
}

type Nachrichten struct {
	Seite       int         `json:"seite"`
	Seiten      int         `json:"seiten"`
	Nachrichten []Nachricht `json:"nachrichten"`
}

type Anhang struct {
	Name   string `json:"name"`
	Inhalt string `json:"inhalt,omitempty"`
	href   string
}

type Beitrag struct {
	Von      string   `json:"von"`
	Datum    string   `json:"datum"`
	Text     string   `json:"text"`
	Anhaenge []Anhang `json:"anhaenge,omitempty"`
}

type Thread struct {
	Betreff   string    `json:"betreff"`
	Fuer      string    `json:"fuer"`
	Beitraege []Beitrag `json:"beitraege"`
}

type Lehrkraft struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Funktion string `json:"funktion,omitempty"`
}

type Lehrkraefte struct {
	Lehrkraefte []Lehrkraft `json:"lehrkraefte"`
}

var (
	threadRe  = regexp.MustCompile(`showMessageFachlehrer\((\d+),\s*(\d+)\)`)
	klammerRe = regexp.MustCompile(`\(([^)]*)\)`)
	idRe      = regexp.MustCompile(`(\d+)/?$`)
)

func (c *Client) Nachrichten(ctx context.Context, seite int) (Nachrichten, error) {
	d, err := c.doc(ctx, fmt.Sprintf("%s?page=%d", fachlehrerPath, max(seite, 1)))
	if err != nil {
		return Nachrichten{}, err
	}
	return parseNachrichten(d)
}

func parseNachrichten(d *goquery.Document) (Nachrichten, error) {
	table := d.Find("table#messages-fachlehrer-table")
	if table.Length() == 0 {
		return Nachrichten{}, fmt.Errorf("%s: Nachrichtentabelle fehlt, Portal-Layout geändert?", fachlehrerPath)
	}
	out := Nachrichten{Seite: 1, Seiten: 1, Nachrichten: []Nachricht{}}
	items := d.Find(".pagination.menu a.item")
	if items.Length() > 0 {
		out.Seiten = items.Length()
		out.Seite = items.IndexOfSelection(items.Filter(".active")) + 1
	}
	table.Find("tbody tr.message-row").Each(func(_ int, tr *goquery.Selection) {
		onclick, _ := tr.Attr("onclick")
		m := threadRe.FindStringSubmatch(onclick)
		if m == nil {
			return
		}
		td := tr.Find("td")
		dir, _ := td.Eq(1).Attr("title")
		lehrer, _ := strconv.Atoi(m[1])
		thread, _ := strconv.Atoi(m[2])
		out.Nachrichten = append(out.Nachrichten, Nachricht{
			LehrerID:     lehrer,
			ThreadID:     thread,
			Lehrkraft:    norm(td.Eq(0).Text()),
			Betreff:      norm(td.Eq(2).Text()),
			Datum:        norm(td.Eq(3).Text()),
			VonLehrkraft: strings.Contains(dir, "Lehrkraft"),
			Anhang:       td.Eq(2).Find("i.paperclip").Length() > 0,
			Ungelesen:    tr.HasClass("unread"),
		})
	})
	return out, nil
}

// Nachricht opens a thread. The portal marks threads read on open, so unread
// ones need ungelesenOeffnen to avoid clearing the marker behind the user's back.
func (c *Client) Nachricht(ctx context.Context, lehrerID, threadID int, ungelesenOeffnen bool) (Thread, error) {
	if !ungelesenOeffnen {
		unread, err := c.ungelesen(ctx, threadID)
		if err != nil {
			return Thread{}, err
		}
		if unread {
			return Thread{}, fmt.Errorf("Thread %d ist ungelesen; Öffnen markiert ihn gelesen. Mit ungelesen_oeffnen=true bestätigen", threadID)
		}
	}
	d, err := c.doc(ctx, fmt.Sprintf("%s/%d/%d", fachlehrerPath, lehrerID, threadID))
	if err != nil {
		return Thread{}, err
	}
	th, err := parseThread(d)
	if err != nil {
		return Thread{}, err
	}
	for i := range th.Beitraege {
		for j := range th.Beitraege[i].Anhaenge {
			a := &th.Beitraege[i].Anhaenge[j]
			if a.Inhalt, err = c.download(ctx, a.href); err != nil {
				return Thread{}, fmt.Errorf("Anhang %s: %w", a.Name, err)
			}
		}
	}
	return th, nil
}

func (c *Client) ungelesen(ctx context.Context, threadID int) (bool, error) {
	for seite, seiten := 1, 1; seite <= seiten; seite++ {
		n, err := c.Nachrichten(ctx, seite)
		if err != nil {
			return false, err
		}
		seiten = n.Seiten
		for _, m := range n.Nachrichten {
			if m.ThreadID == threadID {
				return m.Ungelesen, nil
			}
		}
	}
	return false, fmt.Errorf("Thread %d nicht in der Liste; lehrer_id und thread_id aus list_nachrichten nehmen", threadID)
}

func parseThread(d *goquery.Document) (Thread, error) {
	grid := d.Find("#message-thread-grid")
	if grid.Length() == 0 {
		return Thread{}, errors.New("Thread: #message-thread-grid fehlt, Portal-Layout geändert?")
	}
	th := Thread{Beitraege: []Beitrag{}}
	grid.ChildrenFiltered(".row").Each(func(_ int, row *goquery.Selection) {
		cols := row.ChildrenFiltered(".column")
		left, right := cols.Eq(0), cols.Eq(1)
		label := norm(left.Text())
		switch {
		case label == "Betreff:":
			th.Betreff = norm(right.Text())
		case label == "Für:":
			th.Fuer = norm(right.Text())
		case right.Find(".ui.segment").Length() > 0:
			var datum string
			if m := klammerRe.FindStringSubmatch(label); m != nil {
				datum = m[1]
			}
			th.Beitraege = append(th.Beitraege, Beitrag{
				Von:   norm(left.Find("span.text").Text()),
				Datum: datum,
				Text:  text(right.Find(".ui.segment")),
			})
		case right.Find(`a[href*="get_file"]`).Length() > 0 && len(th.Beitraege) > 0:
			last := &th.Beitraege[len(th.Beitraege)-1]
			right.Find(`a[href*="get_file"]`).Each(func(_ int, a *goquery.Selection) {
				href, _ := a.Attr("href")
				last.Anhaenge = append(last.Anhaenge, Anhang{Name: norm(a.Text()), href: href})
			})
		}
	})
	return th, nil
}

func (c *Client) Lehrkraefte(ctx context.Context) (Lehrkraefte, error) {
	d, err := c.doc(ctx, fachlehrerPath+"/")
	if err != nil {
		return Lehrkraefte{}, err
	}
	return parseLehrkraefte(d)
}

func parseLehrkraefte(d *goquery.Document) (Lehrkraefte, error) {
	cards := d.Find("#messages-fachlehrer-cards .card")
	if cards.Length() == 0 {
		return Lehrkraefte{}, fmt.Errorf("%s/: Lehrkräfte-Liste fehlt, Portal-Layout geändert?", fachlehrerPath)
	}
	out := Lehrkraefte{Lehrkraefte: []Lehrkraft{}}
	cards.Each(func(_ int, card *goquery.Selection) {
		href, _ := card.Find("a[href]").Last().Attr("href")
		m := idRe.FindStringSubmatch(href)
		if m == nil {
			return
		}
		id, _ := strconv.Atoi(m[1])
		out.Lehrkraefte = append(out.Lehrkraefte, Lehrkraft{ID: id, Name: norm(card.Find(".header").Text()), Funktion: norm(card.Find(".meta").Text())})
	})
	return out, nil
}
