package seed

import (
	"os"
	"testing"
)

// The seeded legal drafts must be the reviewed files in docs/legal, word for
// word. To fix, copy each file from docs/legal into backend/seed/legal.
func TestLegalDraftsMatchDocs(t *testing.T) {
	for _, l := range legalPages {
		embedded, err := legalDrafts.ReadFile("legal/" + l.file)
		if err != nil {
			t.Fatal(err)
		}
		docs, err := os.ReadFile("../../docs/legal/" + l.file)
		if err != nil {
			t.Fatal(err)
		}
		if string(embedded) != string(docs) {
			t.Errorf("seed/legal/%s differs from docs/legal/%s; copy the reviewed file into seed/legal", l.file, l.file)
		}
	}
}
