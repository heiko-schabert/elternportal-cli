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

const teacherMessagesPath = "/meldungen/kommunikation_fachlehrer"

type Message struct {
	TeacherID   int    `json:"teacher_id"`
	ThreadID    int    `json:"thread_id"`
	Teacher     string `json:"teacher"`
	Subject     string `json:"subject"`
	Date        string `json:"date"`
	FromTeacher bool   `json:"from_teacher"`
	Attachment  bool   `json:"attachment"`
	Unread      bool   `json:"unread"`
}

type Messages struct {
	Page     int       `json:"page"`
	Pages    int       `json:"pages"`
	Messages []Message `json:"messages"`
}

type Attachment struct {
	Name    string `json:"name"`
	Content string `json:"content,omitempty"`
	href    string
}

type Post struct {
	From        string       `json:"from"`
	Date        string       `json:"date"`
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

type Thread struct {
	Subject string `json:"subject"`
	Child   string `json:"child"`
	Posts   []Post `json:"posts"`
}

type Teacher struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

type Teachers struct {
	Teachers []Teacher `json:"teachers"`
}

var (
	threadRe = regexp.MustCompile(`showMessageFachlehrer\((\d+),\s*(\d+)\)`)
	parenRe  = regexp.MustCompile(`\(([^)]*)\)`)
	idRe     = regexp.MustCompile(`(\d+)/?$`)
)

func (c *Client) Messages(ctx context.Context, page int) (Messages, error) {
	d, err := c.doc(ctx, fmt.Sprintf("%s?page=%d", teacherMessagesPath, max(page, 1)))
	if err != nil {
		return Messages{}, err
	}
	return parseMessages(d)
}

func parseMessages(d *goquery.Document) (Messages, error) {
	table := d.Find("table#messages-fachlehrer-table")
	if table.Length() == 0 {
		return Messages{}, fmt.Errorf("%s: message table missing, portal layout changed?", teacherMessagesPath)
	}
	out := Messages{Page: 1, Pages: 1, Messages: []Message{}}
	items := d.Find(".pagination.menu a.item")
	if items.Length() > 0 {
		out.Pages = items.Length()
		out.Page = items.IndexOfSelection(items.Filter(".active")) + 1
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
		out.Messages = append(out.Messages, Message{
			TeacherID:   lehrer,
			ThreadID:    thread,
			Teacher:     norm(td.Eq(0).Text()),
			Subject:     norm(td.Eq(2).Text()),
			Date:        norm(td.Eq(3).Text()),
			FromTeacher: strings.Contains(dir, "Lehrkraft"),
			Attachment:  td.Eq(2).Find("i.paperclip").Length() > 0,
			Unread:      tr.HasClass("unread"),
		})
	})
	return out, nil
}

// Message opens a thread. The portal marks threads read on open, so unread
// ones need openUnread to avoid clearing the marker behind the user's back.
func (c *Client) Message(ctx context.Context, threadID int, openUnread bool) (Thread, error) {
	m, err := c.findThread(ctx, threadID)
	if err != nil {
		return Thread{}, err
	}
	if m.Unread && !openUnread {
		return Thread{}, fmt.Errorf("thread %d is unread; opening marks it read. Confirm with open_unread=true", threadID)
	}
	d, err := c.doc(ctx, threadPath(m))
	if err != nil {
		return Thread{}, err
	}
	th, err := parseThread(d)
	if err != nil {
		return Thread{}, err
	}
	for i := range th.Posts {
		for j := range th.Posts[i].Attachments {
			a := &th.Posts[i].Attachments[j]
			if a.Content, err = c.download(ctx, a.href); err != nil {
				return Thread{}, fmt.Errorf("attachment %s: %w", a.Name, err)
			}
		}
	}
	return th, nil
}

func threadPath(m Message) string {
	return fmt.Sprintf("%s/%d/%d", teacherMessagesPath, m.TeacherID, m.ThreadID)
}

// findThread looks a thread up in the list; its URL needs the teacher ID,
// which only the list reveals.
func (c *Client) findThread(ctx context.Context, threadID int) (Message, error) {
	for page, pages := 1, 1; page <= pages; page++ {
		n, err := c.Messages(ctx, page)
		if err != nil {
			return Message{}, err
		}
		pages = n.Pages
		for _, m := range n.Messages {
			if m.ThreadID == threadID {
				return m, nil
			}
		}
	}
	return Message{}, fmt.Errorf("thread %d not in the list; take thread_id from the message list", threadID)
}

func parseThread(d *goquery.Document) (Thread, error) {
	grid := d.Find("#message-thread-grid")
	if grid.Length() == 0 {
		return Thread{}, errors.New("thread: #message-thread-grid missing, portal layout changed?")
	}
	th := Thread{Posts: []Post{}}
	grid.ChildrenFiltered(".row").Each(func(_ int, row *goquery.Selection) {
		cols := row.ChildrenFiltered(".column")
		left, right := cols.Eq(0), cols.Eq(1)
		label := norm(left.Text())
		switch {
		case label == "Betreff:":
			th.Subject = norm(right.Text())
		case label == "Für:":
			th.Child = norm(right.Text())
		case right.Find(".ui.segment").Length() > 0:
			var datum string
			if m := parenRe.FindStringSubmatch(label); m != nil {
				datum = m[1]
			}
			th.Posts = append(th.Posts, Post{
				From: norm(left.Find("span.text").Text()),
				Date: datum,
				Text: text(right.Find(".ui.segment")),
			})
		case right.Find(`a[href*="get_file"]`).Length() > 0 && len(th.Posts) > 0:
			last := &th.Posts[len(th.Posts)-1]
			right.Find(`a[href*="get_file"]`).Each(func(_ int, a *goquery.Selection) {
				href, _ := a.Attr("href")
				last.Attachments = append(last.Attachments, Attachment{Name: norm(a.Text()), href: href})
			})
		}
	})
	return th, nil
}

func (c *Client) Teachers(ctx context.Context) (Teachers, error) {
	d, err := c.doc(ctx, teacherMessagesPath+"/")
	if err != nil {
		return Teachers{}, err
	}
	return parseTeachers(d)
}

func parseTeachers(d *goquery.Document) (Teachers, error) {
	cards := d.Find("#messages-fachlehrer-cards .card")
	if cards.Length() == 0 {
		return Teachers{}, fmt.Errorf("%s/: teacher list missing, portal layout changed?", teacherMessagesPath)
	}
	out := Teachers{Teachers: []Teacher{}}
	cards.Each(func(_ int, card *goquery.Selection) {
		href, _ := card.Find("a[href]").Last().Attr("href")
		m := idRe.FindStringSubmatch(href)
		if m == nil {
			return
		}
		id, _ := strconv.Atoi(m[1])
		out.Teachers = append(out.Teachers, Teacher{ID: id, Name: norm(card.Find(".header").Text()), Role: norm(card.Find(".meta").Text())})
	})
	return out, nil
}
