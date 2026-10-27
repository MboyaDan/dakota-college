package letters

import (
	"time"

	"github.com/signintech/gopdf"
)

// Ledger is a finance document (Stage 14a): a receipt, an invoice or a fee
// statement. It is drawn from records only; nothing on it is typed in.

// LedgerColumn is one column of the table. Width is a share of the page's
// text width; the first column wraps, the others are one line.
type LedgerColumn struct {
	Label string
	Width float64
	Right bool
}

type LedgerRow struct {
	Cells []string
	Bold  bool
}

type Ledger struct {
	Title     string // "Receipt", "Invoice", "Fee statement"
	Reference string // shown in the footer
	Date      time.Time
	Details   [][2]string
	Columns   []LedgerColumn
	Rows      []LedgerRow
	// Empty is said instead of the table when there are no rows.
	Empty string
	// Totals are label and value pairs under the table, right-aligned.
	Totals [][2]string
	Notes  string
	Sample bool
}

// LedgerDocument renders a finance document as a PDF.
func LedgerDocument(l Ledger) ([]byte, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4, Unit: gopdf.UnitPT})
	pdf.SetCompressLevel(9)
	pdf.SetInfo(gopdf.PdfInfo{Title: l.Title + " " + l.Reference, Author: "Dakota Training College", Creator: "Dakota Training College finance office", CreationDate: l.Date})
	if err := pdf.AddTTFFontData("regular", interRegular); err != nil {
		return nil, err
	}
	if err := pdf.AddTTFFontData("semibold", interSemiBold); err != nil {
		return nil, err
	}
	w := &writer{pdf: pdf, reference: l.Reference, sample: l.Sample, footerName: l.Title}
	if err := w.newPage(); err != nil {
		return nil, err
	}
	if err := w.letterhead(); err != nil {
		return nil, err
	}

	y := 134.0
	if err := w.text("regular", 10, muted, pageW-margin, y, l.Date.Format("2 January 2006"), gopdf.Right); err != nil {
		return nil, err
	}
	if err := w.text("semibold", 16, navy, margin, y, l.Title, gopdf.Left); err != nil {
		return nil, err
	}
	y += 26
	for _, d := range l.Details {
		if err := w.text("regular", 10, muted, margin, y, d[0], gopdf.Left); err != nil {
			return nil, err
		}
		if err := w.text("semibold", bodySize, ink, margin+120, y, d[1], gopdf.Left); err != nil {
			return nil, err
		}
		y += lineH
	}
	y += 14

	// Column edges: left x of each column, and the right edge.
	lefts := make([]float64, len(l.Columns))
	x := margin
	for i, c := range l.Columns {
		lefts[i] = x
		x += c.Width * contentW
	}
	edge := func(i int) float64 {
		if i == len(l.Columns)-1 {
			return pageW - margin
		}
		return lefts[i+1] - 8
	}
	cell := func(font string, c [3]uint8, i int, y float64, s string) error {
		if l.Columns[i].Right {
			return w.text(font, 9.5, c, edge(i), y, s, gopdf.Right)
		}
		return w.text(font, 9.5, c, lefts[i], y, s, gopdf.Left)
	}
	header := func(y float64) error {
		for i, c := range l.Columns {
			if err := cell("semibold", muted, i, y, c.Label); err != nil {
				return err
			}
		}
		return nil
	}

	if len(l.Rows) == 0 && l.Empty != "" {
		var err error
		if y, err = w.paragraphs(y, l.Empty); err != nil {
			return nil, err
		}
	} else if len(l.Columns) > 0 {
		if err := header(y); err != nil {
			return nil, err
		}
		y += lineH
		for _, row := range l.Rows {
			font := "regular"
			if row.Bold {
				font = "semibold"
			}
			if err := w.pdf.SetFont(font, "", 9.5); err != nil {
				return nil, err
			}
			first, err := w.pdf.SplitTextWithWordWrap(row.Cells[0], edge(0)-lefts[0])
			if err != nil {
				return nil, err
			}
			if y+float64(len(first)+1)*lineH > bottom {
				if err := w.newPage(); err != nil {
					return nil, err
				}
				y = margin
				if err := header(y); err != nil {
					return nil, err
				}
				y += lineH
			}
			w.pdf.SetStrokeColor(stone[0], stone[1], stone[2])
			w.pdf.SetLineWidth(0.75)
			w.pdf.Line(margin, y-3, pageW-margin, y-3)
			for i, s := range row.Cells {
				if i == 0 {
					continue
				}
				if err := cell(font, ink, i, y, s); err != nil {
					return nil, err
				}
			}
			for _, line := range first {
				if err := w.text(font, 9.5, ink, lefts[0], y, line, gopdf.Left); err != nil {
					return nil, err
				}
				y += lineH - 2
			}
			y += 4
		}
	}

	if len(l.Totals) > 0 {
		if y+float64(len(l.Totals)+1)*lineH > bottom {
			if err := w.newPage(); err != nil {
				return nil, err
			}
			y = margin
		}
		y += 6
		w.pdf.SetStrokeColor(navy[0], navy[1], navy[2])
		w.pdf.SetLineWidth(1)
		w.pdf.Line(pageW-margin-220, y-4, pageW-margin, y-4)
		for _, t := range l.Totals {
			if err := w.text("regular", bodySize, muted, pageW-margin-220, y, t[0], gopdf.Left); err != nil {
				return nil, err
			}
			if err := w.text("semibold", bodySize, ink, pageW-margin, y, t[1], gopdf.Right); err != nil {
				return nil, err
			}
			y += lineH + 2
		}
	}

	if l.Notes != "" {
		if _, err := w.paragraphs(y+12, l.Notes); err != nil {
			return nil, err
		}
	}
	return pdf.GetBytesPdf(), nil
}
