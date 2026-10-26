// Package letters makes the college's printable letters as PDFs. The offer
// letter (brief 8) is the first: its wording is an editable template
// (offer.letter); this package adds the logo, date, addressee, fees,
// conditions and sign-off around it.
//
// Fonts are Inter (SIL Open Font License, fonts/OFL.txt), embedded and
// subset into each PDF; the logo is the supplied file from
// docs/brand/dakota-logo, unchanged.
package letters

import (
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/signintech/gopdf"
)

var (
	//go:embed fonts/Inter-Regular.ttf
	interRegular []byte
	//go:embed fonts/Inter-SemiBold.ttf
	interSemiBold []byte
	//go:embed assets/dakota-logo-horizontal.png
	logoPNG []byte
)

// Fee is one line of the fees table.
type Fee struct {
	Label       string
	Frequency   string // once, per_term, per_year, whole_programme
	AmountCents int64
	Notes       string
}

// Offer is everything printed on an offer letter.
type Offer struct {
	Date      time.Time // the letter's date (Nairobi)
	Reference string
	Name      string
	Email     string
	Heading   string // the template subject, filled in
	Body      string // the template body, filled in; blank lines separate paragraphs
	// Conditions of the offer, if any, as the officer typed them.
	Conditions string
	Fees       []Fee
	// Sample marks letters for sample programmes (dev seed), so one is never
	// mistaken for a real offer.
	Sample bool
}

var frequencyLabels = map[string]string{
	"once":            "Once",
	"per_term":        "Per term",
	"per_year":        "Per year",
	"whole_programme": "Whole programme",
}

// Brand colours (CLAUDE.md design tokens).
var (
	navy  = [3]uint8{0x0A, 0x1F, 0x4D}
	ink   = [3]uint8{0x1B, 0x24, 0x30}
	gold  = [3]uint8{0xC9, 0xA2, 0x27}
	ivory = [3]uint8{0xFB, 0xF8, 0xF1}
	stone = [3]uint8{0xE5, 0xE0, 0xD5}
	muted = [3]uint8{0x55, 0x5E, 0x6B}
)

const (
	pageW, pageH = 595.0, 842.0 // A4 in points
	margin       = 56.0
	contentW     = pageW - 2*margin
	bottom       = pageH - 72.0 // keep clear of the footer
	bodySize     = 10.5
	lineH        = 15.0
)

// FormatKES formats cents as "KES 45,000.00" without floating point.
func FormatKES(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	whole := fmt.Sprint(cents / 100)
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("KES %s%s.%02d", sign, b.String(), cents%100)
}

type writer struct {
	pdf       *gopdf.GoPdf
	reference string
	page      int
	sample    bool
}

// OfferLetter renders the letter as a PDF.
func OfferLetter(o Offer) ([]byte, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4, Unit: gopdf.UnitPT})
	pdf.SetCompressLevel(9)
	pdf.SetInfo(gopdf.PdfInfo{
		Title: o.Heading + " " + o.Reference, Author: "Dakota Training College", Creator: "Dakota Training College admissions",
		CreationDate: o.Date,
	})
	if err := pdf.AddTTFFontData("regular", interRegular); err != nil {
		return nil, err
	}
	if err := pdf.AddTTFFontData("semibold", interSemiBold); err != nil {
		return nil, err
	}
	w := &writer{pdf: pdf, reference: o.Reference, sample: o.Sample}
	if err := w.newPage(); err != nil {
		return nil, err
	}

	// The supplied logo has an ivory background; the letterhead band is the
	// same ivory, so the logo sits on its own colour.
	pdf.SetFillColor(ivory[0], ivory[1], ivory[2])
	pdf.RectFromUpperLeftWithStyle(0, 0, pageW, 112, "F")
	logo, err := gopdf.ImageHolderByBytes(logoPNG)
	if err != nil {
		return nil, err
	}
	// 2400 x 792 px, shown 190 pt wide.
	if err := pdf.ImageByHolder(logo, margin, 24, &gopdf.Rect{W: 190, H: 190 * 792 / 2400}); err != nil {
		return nil, err
	}
	pdf.SetStrokeColor(gold[0], gold[1], gold[2])
	pdf.SetLineWidth(1.5)
	pdf.Line(margin, 112, pageW-margin, 112)

	y := 134.0
	if err := w.text("regular", 10, muted, pageW-margin, y, o.Date.Format("2 January 2006"), gopdf.Right); err != nil {
		return nil, err
	}
	if err := w.text("semibold", bodySize, ink, margin, y, o.Name, gopdf.Left); err != nil {
		return nil, err
	}
	y += lineH
	if o.Email != "" {
		if err := w.text("regular", bodySize, ink, margin, y, o.Email, gopdf.Left); err != nil {
			return nil, err
		}
		y += lineH
	}
	y += 6
	if err := w.text("regular", 10, muted, margin, y, "Application reference: "+o.Reference, gopdf.Left); err != nil {
		return nil, err
	}
	y += 30

	if err := w.text("semibold", 16, navy, margin, y, o.Heading, gopdf.Left); err != nil {
		return nil, err
	}
	y += 28

	if y, err = w.paragraphs(y, o.Body); err != nil {
		return nil, err
	}

	if c := strings.TrimSpace(o.Conditions); c != "" {
		if y, err = w.heading(y+6, "Conditions of this offer"); err != nil {
			return nil, err
		}
		if y, err = w.paragraphs(y, c); err != nil {
			return nil, err
		}
	}

	if len(o.Fees) > 0 {
		if y, err = w.fees(y+6, o.Fees, o.Date); err != nil {
			return nil, err
		}
	}

	if y+4*lineH > bottom {
		if err := w.newPage(); err != nil {
			return nil, err
		}
		y = margin
	}
	y += 10
	for i, line := range []string{"Yours sincerely,", "", "Admissions Office", "Dakota Training College"} {
		font := "regular"
		if i >= 2 {
			font = "semibold"
		}
		if err := w.text(font, bodySize, ink, margin, y, line, gopdf.Left); err != nil {
			return nil, err
		}
		y += lineH
	}
	return pdf.GetBytesPdf(), nil
}

func (w *writer) newPage() error {
	w.pdf.AddPage()
	w.page++
	footer := fmt.Sprintf("Dakota Training College  ·  Offer letter %s  ·  Page %d", w.reference, w.page)
	if err := w.text("regular", 8, muted, margin, pageH-40, footer, gopdf.Left); err != nil {
		return err
	}
	if w.sample {
		return w.text("semibold", 9, [3]uint8{0xB4, 0x23, 0x18}, pageW-margin, pageH-40, "SAMPLE: NOT A REAL OFFER", gopdf.Right)
	}
	return nil
}

// text draws one line with its top at y; align is gopdf.Left or gopdf.Right
// (x is then the right edge).
func (w *writer) text(font string, size float64, c [3]uint8, x, y float64, s string, align int) error {
	if err := w.pdf.SetFont(font, "", size); err != nil {
		return err
	}
	w.pdf.SetTextColor(c[0], c[1], c[2])
	if align == gopdf.Right {
		width, err := w.pdf.MeasureTextWidth(s)
		if err != nil {
			return err
		}
		x -= width
	}
	w.pdf.SetXY(x, y)
	return w.pdf.Cell(nil, s)
}

func (w *writer) heading(y float64, s string) (float64, error) {
	if y+3*lineH > bottom {
		if err := w.newPage(); err != nil {
			return 0, err
		}
		y = margin
	}
	if err := w.text("semibold", 11.5, navy, margin, y, s, gopdf.Left); err != nil {
		return 0, err
	}
	return y + lineH + 4, nil
}

// paragraphs wraps text to the page, starting new pages as needed. Blank
// lines separate paragraphs; single line breaks are kept.
func (w *writer) paragraphs(y float64, text string) (float64, error) {
	text = strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n")
	if err := w.pdf.SetFont("regular", "", bodySize); err != nil {
		return 0, err
	}
	for _, para := range strings.Split(text, "\n\n") {
		for _, raw := range strings.Split(para, "\n") {
			lines := []string{""}
			if strings.TrimSpace(raw) != "" {
				var err error
				if lines, err = w.pdf.SplitTextWithWordWrap(raw, contentW); err != nil {
					return 0, err
				}
			}
			for _, line := range lines {
				if y+lineH > bottom {
					if err := w.newPage(); err != nil {
						return 0, err
					}
					y = margin
				}
				if err := w.text("regular", bodySize, ink, margin, y, line, gopdf.Left); err != nil {
					return 0, err
				}
				y += lineH
			}
		}
		y += lineH * 0.6
	}
	return y, nil
}

func (w *writer) fees(y float64, fees []Fee, date time.Time) (float64, error) {
	y, err := w.heading(y, "Fees")
	if err != nil {
		return 0, err
	}
	colFreq, colAmount := margin+contentW*0.55, pageW-margin
	row := func(y float64, font string, c [3]uint8, label, freq, amount string) error {
		if err := w.text(font, 9.5, c, margin, y, label, gopdf.Left); err != nil {
			return err
		}
		if err := w.text(font, 9.5, c, colFreq, y, freq, gopdf.Left); err != nil {
			return err
		}
		return w.text(font, 9.5, c, colAmount, y, amount, gopdf.Right)
	}
	if err := row(y, "semibold", muted, "Item", "How often", "Amount"); err != nil {
		return 0, err
	}
	y += lineH
	w.pdf.SetStrokeColor(stone[0], stone[1], stone[2])
	w.pdf.SetLineWidth(0.75)
	for _, f := range fees {
		if y+2*lineH > bottom {
			if err := w.newPage(); err != nil {
				return 0, err
			}
			y = margin
		}
		w.pdf.Line(margin, y-3, pageW-margin, y-3)
		if err := w.pdf.SetFont("regular", "", 9.5); err != nil {
			return 0, err
		}
		// Long labels wrap within their column.
		lines, err := w.pdf.SplitTextWithWordWrap(f.Label, colFreq-margin-12)
		if err != nil {
			return 0, err
		}
		freq := frequencyLabels[f.Frequency]
		if freq == "" {
			freq = f.Frequency
		}
		if err := row(y, "regular", ink, lines[0], freq, FormatKES(f.AmountCents)); err != nil {
			return 0, err
		}
		for _, l := range lines[1:] {
			y += lineH - 2
			if err := w.text("regular", 9.5, ink, margin, y, l, gopdf.Left); err != nil {
				return 0, err
			}
		}
		// Notes go under the item, smaller and muted.
		if f.Notes != "" {
			if err := w.pdf.SetFont("regular", "", 8.5); err != nil {
				return 0, err
			}
			notes, err := w.pdf.SplitTextWithWordWrap(f.Notes, colFreq-margin-12)
			if err != nil {
				return 0, err
			}
			for _, l := range notes {
				y += lineH - 3
				if err := w.text("regular", 8.5, muted, margin, y, l, gopdf.Left); err != nil {
					return 0, err
				}
			}
		}
		y += lineH + 2
	}
	note := fmt.Sprintf("These are the fees published for this programme on %s. Your invoice at enrolment confirms what you pay.",
		date.Format("2 January 2006"))
	return w.paragraphs(y+4, note)
}
