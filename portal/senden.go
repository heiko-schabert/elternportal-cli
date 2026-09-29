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
	fachlehrerInsPath = "/includes/meldungen_dir/kommunikation_fachlehrer_db_ins.php"
	klassenleitungNeu = "/meldungen/kommunikation/neu"
	klassenleitungIns = "/includes/meldungen_dir/kommunikation_db_ins.php"
)

// formCSRF reads the token of the page's form; each form page issues its own.
func formCSRF(d *goquery.Document, path string) (string, error) {
	csrf, ok := d.Find(`form input[name="csrf"]`).Attr("value")
	if !ok {
		return "", fmt.Errorf("%s: Formular/CSRF fehlt, Portal-Layout geändert?", path)
	}
	return csrf, nil
}

// Antworten replies in a thread and returns the reloaded thread as proof.
func (c *Client) Antworten(ctx context.Context, lehrerID, threadID int, text string) (Thread, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Thread{}, errors.New("text leer")
	}
	path := fmt.Sprintf("%s/%d/%d", fachlehrerPath, lehrerID, threadID)
	d, err := c.doc(ctx, path)
	if err != nil {
		return Thread{}, err
	}
	csrf, err := formCSRF(d, path)
	if err != nil {
		return Thread{}, err
	}
	if err := c.postMultipart(ctx, fachlehrerInsPath, map[string]string{
		"csrf": csrf, "nachricht_kom_fach": text,
		"kob_id": strconv.Itoa(threadID), "teacher_id": strconv.Itoa(lehrerID),
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
	if n := len(th.Beitraege); n == 0 || norm(th.Beitraege[n-1].Text) != norm(text) {
		return th, errors.New("gesendet, aber Nachricht nicht im Thread sichtbar; im Portal prüfen")
	}
	return th, nil
}

// NeueNachricht starts a thread and returns it from the refreshed list.
func (c *Client) NeueNachricht(ctx context.Context, lehrerID int, betreff, text string) (Nachricht, error) {
	// The list shows the subject whitespace-collapsed; send it that way to find it again.
	betreff, text = norm(betreff), strings.TrimSpace(text)
	switch {
	case betreff == "" || text == "":
		return Nachricht{}, errors.New("betreff und text nötig")
	case utf8.RuneCountInString(betreff) > 128:
		return Nachricht{}, errors.New("betreff max. 128 Zeichen")
	}
	path := fmt.Sprintf("%s/%d", fachlehrerPath, lehrerID)
	d, err := c.doc(ctx, path)
	if err != nil {
		return Nachricht{}, err
	}
	csrf, err := formCSRF(d, path)
	if err != nil {
		return Nachricht{}, err
	}
	if err := c.postMultipart(ctx, fachlehrerInsPath, map[string]string{
		"csrf": csrf, "teacher_id": strconv.Itoa(lehrerID),
		"new_betreff": betreff, "nachricht_kom_fach": text,
	}); err != nil {
		return Nachricht{}, err
	}
	n, err := c.Nachrichten(ctx, 1)
	if err != nil {
		return Nachricht{}, err
	}
	for _, m := range n.Nachrichten {
		if m.Betreff == betreff {
			return m, nil
		}
	}
	return Nachricht{}, errors.New("gesendet, aber Thread nicht in Liste sichtbar; im Portal prüfen")
}

// KlassenleitungAnfrage sends a contact request; the portal only offers
// fixed reasons, matched case-insensitively.
func (c *Client) KlassenleitungAnfrage(ctx context.Context, kontaktwunsch, grund string) error {
	grund = strings.TrimSpace(grund)
	if grund == "" {
		return errors.New("grund leer")
	}
	d, err := c.doc(ctx, klassenleitungNeu)
	if err != nil {
		return err
	}
	csrf, err := formCSRF(d, klassenleitungNeu)
	if err != nil {
		return err
	}
	var opts []string
	betreff := ""
	d.Find(`select[name="betreff"] option`).Each(func(_ int, o *goquery.Selection) {
		opt := norm(o.Text())
		opts = append(opts, opt)
		if strings.EqualFold(opt, strings.TrimSpace(kontaktwunsch)) {
			betreff = opt
		}
	})
	if betreff == "" {
		return fmt.Errorf("kontaktwunsch %q ungültig, erlaubt: %s", kontaktwunsch, strings.Join(opts, ", "))
	}
	return c.postMultipart(ctx, klassenleitungIns, map[string]string{"csrf": csrf, "betreff": betreff, "kommentar": grund})
}
