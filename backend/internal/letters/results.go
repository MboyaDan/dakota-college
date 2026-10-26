package letters

import (
	"fmt"
	"time"

	"github.com/signintech/gopdf"
)

// ResultRow is one unit on a results document.
type ResultRow struct {
	UnitCode  string
	UnitTitle string
	Mark      string // total, or empty for competency units
	Grade     string // grade, or Competent / Not yet competent
}

// ResultTerm groups the units of one term.
type ResultTerm struct {
	Name string
	Rows []ResultRow
}

// Results is a results slip (provisional, any student) or an official
// transcript (issued by the registrar's office, numbered).
type Results struct {
	Official      bool
	Number        string // transcript number, official only
	Date          time.Time
	Name          string
	StudentNumber string
	Programme     string
	Terms         []ResultTerm
	Sample        bool
}

// ResultsDocument renders released results as a PDF. Only released results
// are passed in (AC-09); the document says plainly which kind it is.
func ResultsDocument(r Results) ([]byte, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4, Unit: gopdf.UnitPT})
	pdf.SetCompressLevel(9)
	title := "Results slip"
	if r.Official {
		title = "Official transcript"
	}
	pdf.SetInfo(gopdf.PdfInfo{Title: title + " " + r.StudentNumber, Author: "Dakota Training College", Creator: "Dakota Training College registrar's office", CreationDate: r.Date})
	if err := pdf.AddTTFFontData("regular", interRegular); err != nil {
		return nil, err
	}
	if err := pdf.AddTTFFontData("semibold", interSemiBold); err != nil {
		return nil, err
	}
	ref := r.StudentNumber
	if r.Official {
		ref = r.Number
	}
	w := &writer{pdf: pdf, reference: ref, sample: r.Sample, footerName: title}
	if err := w.newPage(); err != nil {
		return nil, err
	}
	if err := w.letterhead(); err != nil {
		return nil, err
	}

	y := 134.0
	if err := w.text("regular", 10, muted, pageW-margin, y, r.Date.Format("2 January 2006"), gopdf.Right); err != nil {
		return nil, err
	}
	if err := w.text("semibold", 16, navy, margin, y, title, gopdf.Left); err != nil {
		return nil, err
	}
	y += 26
	details := [][2]string{{"Name", r.Name}, {"Student number", r.StudentNumber}, {"Programme", r.Programme}}
	if r.Official {
		details = append(details, [2]string{"Transcript number", r.Number})
	}
	for _, d := range details {
		if err := w.text("regular", 10, muted, margin, y, d[0], gopdf.Left); err != nil {
			return nil, err
		}
		if err := w.text("semibold", bodySize, ink, margin+110, y, d[1], gopdf.Left); err != nil {
			return nil, err
		}
		y += lineH
	}
	y += 10
	if !r.Official {
		var err error
		if y, err = w.paragraphs(y, "Not an official transcript. It lists the results released to you so far. Ask the registrar's office for an official transcript."); err != nil {
			return nil, err
		}
	}

	colMark, colGrade := pageW-margin-110, pageW-margin
	if len(r.Terms) == 0 {
		var err error
		if y, err = w.paragraphs(y+6, "No results have been released yet."); err != nil {
			return nil, err
		}
	}
	for _, t := range r.Terms {
		var err error
		if y, err = w.heading(y+6, t.Name); err != nil {
			return nil, err
		}
		header := [][3]any{{margin, "Unit", gopdf.Left}, {colMark, "Mark", gopdf.Right}, {colGrade, "Result", gopdf.Right}}
		for _, h := range header {
			if err := w.text("semibold", 9.5, muted, h[0].(float64), y, h[1].(string), h[2].(int)); err != nil {
				return nil, err
			}
		}
		y += lineH
		w.pdf.SetStrokeColor(stone[0], stone[1], stone[2])
		w.pdf.SetLineWidth(0.75)
		for _, row := range t.Rows {
			if y+2*lineH > bottom {
				if err := w.newPage(); err != nil {
					return nil, err
				}
				y = margin
			}
			w.pdf.Line(margin, y-3, pageW-margin, y-3)
			if err := w.pdf.SetFont("regular", "", 9.5); err != nil {
				return nil, err
			}
			lines, err := w.pdf.SplitTextWithWordWrap(row.UnitCode+"  "+row.UnitTitle, colMark-margin-70)
			if err != nil {
				return nil, err
			}
			for i, l := range lines {
				if err := w.text("regular", 9.5, ink, margin, y, l, gopdf.Left); err != nil {
					return nil, err
				}
				if i == 0 {
					if err := w.text("regular", 9.5, ink, colMark, y, row.Mark, gopdf.Right); err != nil {
						return nil, err
					}
					if err := w.text("semibold", 9.5, ink, colGrade, y, row.Grade, gopdf.Right); err != nil {
						return nil, err
					}
				}
				y += lineH - 2
			}
			y += 4
		}
	}
	if r.Official {
		if y+4*lineH > bottom {
			if err := w.newPage(); err != nil {
				return nil, err
			}
			y = margin
		}
		y += 24
		for i, line := range []string{"Registrar", "Dakota Training College"} {
			font := "semibold"
			if i == 0 {
				font = "regular"
			}
			if err := w.text(font, bodySize, ink, margin, y, line, gopdf.Left); err != nil {
				return nil, err
			}
			y += lineH
		}
	}
	return pdf.GetBytesPdf(), nil
}

// footerLabel names the document in the footer.
func (w *writer) footerLabel() string {
	if w.footerName != "" {
		return fmt.Sprintf("Dakota Training College  ·  %s %s  ·  Page %d", w.footerName, w.reference, w.page)
	}
	return fmt.Sprintf("Dakota Training College  ·  Offer letter %s  ·  Page %d", w.reference, w.page)
}
