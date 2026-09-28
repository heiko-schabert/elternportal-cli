package portal

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const elternbriefePath = "/aktuelles/elternbriefe"

type Elternbrief struct {
	Nummer     int    `json:"nummer"`
	Titel      string `json:"titel"`
	Datum      string `json:"datum"`
	Klassen    string `json:"klassen"`
	Bestaetigt bool   `json:"bestaetigt"`
	HatDatei   bool   `json:"hat_datei"`

	downloadURL string
	inline      string
}

type Elternbriefe struct {
	Briefe []Elternbrief `json:"briefe"`
}

type ElternbriefInhalt struct {
	Elternbrief
	Inhalt string `json:"inhalt"`
}

var (
	nrRe      = regexp.MustCompile(`#(\d+)`)
	zeitRe    = regexp.MustCompile(`\d{2}\.\d{2}\.\d{4},?\s+\d{2}:\d{2}(:\d{2})?`)
	klassenRe = regexp.MustCompile(`Klasse/n:\s*(.+)`)
)

func (c *Client) Elternbriefe(ctx context.Context) (Elternbriefe, error) {
	s, err := c.page(ctx, elternbriefePath)
	if err != nil {
		return Elternbriefe{}, err
	}
	return Elternbriefe{Briefe: parseElternbriefe(s)}, nil
}

// parseElternbriefe reads row pairs: a header row with "#Nr" and status,
// then a body row with title, date, classes, an optional file link and the
// letter text as paragraphs.
func parseElternbriefe(s *goquery.Selection) []Elternbrief {
	out := []Elternbrief{}
	rows := s.Find("table.ui tr")
	for i := 0; i+1 < rows.Length(); i++ {
		head := rows.Eq(i).Text()
		m := nrRe.FindStringSubmatch(head)
		if m == nil {
			continue
		}
		body := rows.Eq(i + 1)
		i++
		nr, _ := strconv.Atoi(m[1])
		b := Elternbrief{Nummer: nr, Bestaetigt: !strings.Contains(head, "noch nicht")}
		el := body.Find("a.link_nachrichten").First()
		if el.Length() > 0 {
			b.HatDatei = true
			b.downloadURL, _ = el.Attr("href")
		} else {
			el = body.Find("span.link_nachrichten").First()
		}
		b.Titel = strings.TrimSpace(el.Find("h4").Text())
		b.Datum = zeitRe.FindString(text(el))
		if m := klassenRe.FindStringSubmatch(body.Find("span.small").Text()); m != nil {
			b.Klassen = strings.TrimSpace(m[1])
		}
		b.inline = text(body.Find("p"))
		out = append(out, b)
	}
	return out
}

func findBrief(bs []Elternbrief, nummer int, titel string) (Elternbrief, error) {
	var hit *Elternbrief
	switch {
	case nummer > 0:
		for i := range bs {
			if bs[i].Nummer == nummer {
				hit = &bs[i]
				break
			}
		}
	case titel != "":
		needle := strings.ToLower(titel)
		for i := range bs {
			if strings.Contains(strings.ToLower(bs[i].Titel), needle) && (hit == nil || bs[i].Nummer > hit.Nummer) {
				hit = &bs[i]
			}
		}
	default:
		return Elternbrief{}, errors.New("nummer oder titel angeben")
	}
	if hit != nil {
		return *hit, nil
	}
	var avail []string
	for _, b := range bs[:min(10, len(bs))] {
		avail = append(avail, fmt.Sprintf("#%d %s", b.Nummer, b.Titel))
	}
	return Elternbrief{}, fmt.Errorf("Elternbrief nicht gefunden; neueste: %s", strings.Join(avail, "; "))
}

func (c *Client) Elternbrief(ctx context.Context, nummer int, titel string) (ElternbriefInhalt, error) {
	s, err := c.page(ctx, elternbriefePath)
	if err != nil {
		return ElternbriefInhalt{}, err
	}
	b, err := findBrief(parseElternbriefe(s), nummer, titel)
	if err != nil {
		return ElternbriefInhalt{}, err
	}
	out := ElternbriefInhalt{Elternbrief: b, Inhalt: b.inline}
	if !b.HatDatei {
		return out, nil
	}
	path, err := c.resolve(b.downloadURL)
	if err != nil {
		return ElternbriefInhalt{}, err
	}
	raw, ct, err := c.fetch(ctx, path)
	if err != nil {
		return ElternbriefInhalt{}, err
	}
	if strings.Contains(ct, "html") {
		return ElternbriefInhalt{}, fmt.Errorf("Elternbrief #%d: Download lieferte HTML statt Datei", b.Nummer)
	}
	pdf, err := pdfText(ctx, raw)
	if errors.Is(err, exec.ErrNotFound) {
		pdf = "(pdftotext nicht installiert, PDF-Text nicht verfügbar)"
	} else if err != nil {
		return ElternbriefInhalt{}, err
	}
	out.Inhalt = strings.TrimSpace(b.inline + "\n\n" + pdf)
	return out, nil
}
