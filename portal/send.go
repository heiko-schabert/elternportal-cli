package portal

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
)

const (
	teacherMessagesInsPath = "/includes/meldungen_dir/kommunikation_fachlehrer_db_ins.php"
	classTeacherNewPath    = "/meldungen/kommunikation/neu"
	classTeacherInsPath    = "/includes/meldungen_dir/kommunikation_db_ins.php"
)

// formCSRF reads the token of the page's form; each form page issues its own.
func formCSRF(d *goquery.Document, path string) (string, error) {
	csrf, ok := d.Find(`form input[name="csrf"]`).Attr("value")
	if !ok {
		return "", fmt.Errorf("%s: form/CSRF missing, portal layout changed?", path)
	}
	return csrf, nil
}

// Reply replies in a thread and returns the reloaded thread as proof.
func (c *Client) Reply(ctx context.Context, threadID int, text string) (Thread, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Thread{}, errors.New("text empty")
	}
	m, err := c.findThread(ctx, threadID)
	if err != nil {
		return Thread{}, err
	}
	path := threadPath(m)
	d, err := c.doc(ctx, path)
	if err != nil {
		return Thread{}, err
	}
	csrf, err := formCSRF(d, path)
	if err != nil {
		return Thread{}, err
	}
	if err := c.postMultipart(ctx, teacherMessagesInsPath, map[string]string{
		"csrf": csrf, "nachricht_kom_fach": text,
		"kob_id": strconv.Itoa(threadID), "teacher_id": strconv.Itoa(m.TeacherID),
	}); err != nil {
		return Thread{}, err
	}
	if d, err = c.doc(ctx, path); err != nil {
		return Thread{}, err
	}
	th, err := parseThread(d)
	if err != nil {
		return Thread{}, err
	}
	// The portal renders line breaks as <br>, so compare whitespace-insensitively.
	if n := len(th.Posts); n == 0 || norm(th.Posts[n-1].Text) != norm(text) {
		return th, errors.New("sent, but message not visible in the thread; check the portal")
	}
	return th, nil
}

// NewMessage starts a thread and returns it from the refreshed list.
func (c *Client) NewMessage(ctx context.Context, teacherID int, subject, text string) (Message, error) {
	// The list shows the subject whitespace-collapsed; send it that way to find it again.
	subject, text = norm(subject), strings.TrimSpace(text)
	switch {
	case subject == "" || text == "":
		return Message{}, errors.New("subject and text required")
	case utf8.RuneCountInString(subject) > 128:
		return Message{}, errors.New("subject max. 128 characters")
	}
	path := fmt.Sprintf("%s/%d", teacherMessagesPath, teacherID)
	d, err := c.doc(ctx, path)
	if err != nil {
		return Message{}, err
	}
	csrf, err := formCSRF(d, path)
	if err != nil {
		return Message{}, err
	}
	if err := c.postMultipart(ctx, teacherMessagesInsPath, map[string]string{
		"csrf": csrf, "teacher_id": strconv.Itoa(teacherID),
		"new_betreff": subject, "nachricht_kom_fach": text,
	}); err != nil {
		return Message{}, err
	}
	n, err := c.Messages(ctx, 1)
	if err != nil {
		return Message{}, err
	}
	for _, m := range n.Messages {
		if m.Subject == subject {
			return m, nil
		}
	}
	return Message{}, errors.New("sent, but thread not visible in the list; check the portal")
}

// ContactClassTeacher sends a contact request; the portal only offers
// fixed reasons, matched case-insensitively.
func (c *Client) ContactClassTeacher(ctx context.Context, requestType, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("reason empty")
	}
	d, err := c.doc(ctx, classTeacherNewPath)
	if err != nil {
		return err
	}
	csrf, err := formCSRF(d, classTeacherNewPath)
	if err != nil {
		return err
	}
	var opts []string
	subject := ""
	d.Find(`select[name="betreff"] option`).Each(func(_ int, o *goquery.Selection) {
		opt := norm(o.Text())
		opts = append(opts, opt)
		if strings.EqualFold(opt, strings.TrimSpace(requestType)) {
			subject = opt
		}
	})
	if subject == "" {
		return fmt.Errorf("type %q invalid, allowed: %s", requestType, strings.Join(opts, ", "))
	}
	return c.postMultipart(ctx, classTeacherInsPath, map[string]string{"csrf": csrf, "betreff": subject, "kommentar": reason})
}
