package portal

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const lettersPath = "/aktuelles/elternbriefe"

type Letter struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Date      string `json:"date"`
	Classes   string `json:"classes"`
	Confirmed bool   `json:"confirmed"`
	HasFile   bool   `json:"has_file"`

	downloadURL string
	inline      string
	id          string // confirmation ID from the header cell "empf_<id>"
}

type Letters struct {
	Letters []Letter `json:"letters"`
}

type LetterContent struct {
	Letter
	Content string `json:"content"`
}

var (
	numberRe  = regexp.MustCompile(`#(\d+)`)
	timeRe    = regexp.MustCompile(`\d{2}\.\d{2}\.\d{4},?\s+\d{2}:\d{2}(:\d{2})?`)
	classesRe = regexp.MustCompile(`Klasse/n:\s*(.+)`)
)

func (c *Client) Letters(ctx context.Context) (Letters, error) {
	s, err := c.page(ctx, lettersPath)
	if err != nil {
		return Letters{}, err
	}
	return Letters{Letters: parseLetters(s)}, nil
}

// parseLetters reads row pairs: a header row with "#Nr" and status,
// then a body row with title, date, classes, an optional file link and the
// letter text as paragraphs.
func parseLetters(s *goquery.Selection) []Letter {
	out := []Letter{}
	rows := s.Find("table.ui tr")
	for i := 0; i+1 < rows.Length(); i++ {
		head := rows.Eq(i).Text()
		m := numberRe.FindStringSubmatch(head)
		if m == nil {
			continue
		}
		body := rows.Eq(i + 1)
		i++
		nr, _ := strconv.Atoi(m[1])
		b := Letter{Number: nr, Confirmed: !strings.Contains(head, "noch nicht")}
		id, _ := rows.Eq(i - 1).Find(`td[id^="empf_"]`).Attr("id")
		b.id = strings.TrimPrefix(id, "empf_")
		el := body.Find("a.link_nachrichten").First()
		if el.Length() > 0 {
			b.HasFile = true
			b.downloadURL, _ = el.Attr("href")
		} else {
			el = body.Find("span.link_nachrichten").First()
		}
		b.Title = strings.TrimSpace(el.Find("h4").Text())
		b.Date = timeRe.FindString(text(el))
		if m := classesRe.FindStringSubmatch(body.Find("span.small").Text()); m != nil {
			b.Classes = strings.TrimSpace(m[1])
		}
		b.inline = text(body.Find("p"))
		out = append(out, b)
	}
	return out
}

func findLetter(bs []Letter, number int, title string) (Letter, error) {
	var hit *Letter
	switch {
	case number > 0:
		for i := range bs {
			if bs[i].Number == number {
				hit = &bs[i]
				break
			}
		}
	case title != "":
		needle := strings.ToLower(title)
		for i := range bs {
			if strings.Contains(strings.ToLower(bs[i].Title), needle) && (hit == nil || bs[i].Number > hit.Number) {
				hit = &bs[i]
			}
		}
	default:
		return Letter{}, errors.New("pass number or title")
	}
	if hit != nil {
		return *hit, nil
	}
	var avail []string
	for _, b := range bs[:min(10, len(bs))] {
		avail = append(avail, fmt.Sprintf("#%d %s", b.Number, b.Title))
	}
	return Letter{}, fmt.Errorf("letter not found; newest: %s", strings.Join(avail, "; "))
}

func (c *Client) Letter(ctx context.Context, number int, title string) (LetterContent, error) {
	s, err := c.page(ctx, lettersPath)
	if err != nil {
		return LetterContent{}, err
	}
	b, err := findLetter(parseLetters(s), number, title)
	if err != nil {
		return LetterContent{}, err
	}
	out := LetterContent{Letter: b, Content: b.inline}
	if !b.HasFile {
		return out, nil
	}
	pdf, err := c.download(ctx, b.downloadURL)
	if err != nil {
		return LetterContent{}, fmt.Errorf("letter #%d: %w", b.Number, err)
	}
	out.Content = strings.TrimSpace(b.inline + "\n\n" + pdf)
	return out, nil
}

// ConfirmLetter confirms receipt and reloads the list, so the
// returned status is what the portal now shows.
func (c *Client) ConfirmLetter(ctx context.Context, number int) (Letter, error) {
	s, err := c.page(ctx, lettersPath)
	if err != nil {
		return Letter{}, err
	}
	b, err := findLetter(parseLetters(s), number, "")
	if err != nil || b.Confirmed {
		return b, err
	}
	if b.id == "" {
		return Letter{}, fmt.Errorf("letter #%d: confirmation ID missing", number)
	}
	if _, _, err := c.fetch(ctx, "/api/elternbrief_bestaetigen.php?eb="+url.QueryEscape(b.id)); err != nil {
		return Letter{}, err
	}
	if s, err = c.page(ctx, lettersPath); err != nil {
		return Letter{}, err
	}
	b, err = findLetter(parseLetters(s), number, "")
	if err == nil && !b.Confirmed {
		err = fmt.Errorf("letter #%d: portal still shows it unconfirmed", number)
	}
	return b, err
}
