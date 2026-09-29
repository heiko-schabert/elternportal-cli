package portal

import (
	"context"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Substitution struct {
	Lesson           string `json:"lesson"`
	Teacher          string `json:"teacher"`
	Substitution     string `json:"substitute"`
	CancelledSubject string `json:"cancelled_subject,omitempty"`
	Subject          string `json:"subject"`
	Room             string `json:"room"`
	Info             string `json:"info"`
}

type Day struct {
	Date          string         `json:"date"`
	Week          string         `json:"week"`
	Substitutions []Substitution `json:"substitutions"`
}

type SubstitutionPlan struct {
	Updated string `json:"updated"`
	Days    []Day  `json:"days"`
}

var updatedRe = regexp.MustCompile(`Stand:\s*(.+)`)

func (c *Client) SubstitutionPlan(ctx context.Context) (SubstitutionPlan, error) {
	s, err := c.page(ctx, "/service/vertretungsplan")
	if err != nil {
		return SubstitutionPlan{}, err
	}
	return parseSubstitutionPlan(s), nil
}

// norm collapses whitespace including &nbsp;, which the portal pads cells with.
func norm(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// parseSubstitutionPlan walks day headers and tables in document order; each
// table belongs to the preceding "Date - Week" header.
func parseSubstitutionPlan(s *goquery.Selection) SubstitutionPlan {
	vp := SubstitutionPlan{Days: []Day{}}
	s.Find("div.list, table").Each(func(_ int, el *goquery.Selection) {
		if goquery.NodeName(el) != "table" {
			t := norm(el.Text())
			if m := updatedRe.FindStringSubmatch(t); m != nil {
				vp.Updated = m[1]
			} else if el.HasClass("bold") {
				datum, kw, _ := strings.Cut(t, " - ")
				vp.Days = append(vp.Days, Day{Date: datum, Week: kw, Substitutions: []Substitution{}})
			}
			return
		}
		if len(vp.Days) == 0 {
			return
		}
		tag := &vp.Days[len(vp.Days)-1]
		el.Find("tr").Each(func(_ int, tr *goquery.Selection) {
			td := tr.Find("td")
			if td.Length() < 6 || td.HasClass("table_header") {
				return
			}
			// A struck-through subject is the cancelled one; the rest is the replacement.
			fach := td.Eq(3)
			alt := fach.Find(`span[style*="line-through"]`).Text()
			tag.Substitutions = append(tag.Substitutions, Substitution{
				Lesson:           norm(td.Eq(0).Text()),
				Teacher:          norm(td.Eq(1).Text()),
				Substitution:     norm(td.Eq(2).Text()),
				CancelledSubject: norm(alt),
				Subject:          norm(strings.Replace(fach.Text(), alt, "", 1)),
				Room:             norm(td.Eq(4).Text()),
				Info:             norm(td.Eq(5).Text()),
			})
		})
	})
	return vp
}
