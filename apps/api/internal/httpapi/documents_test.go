package httpapi

import (
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"testing"
)

func TestDocumentDatesAndSignatureConfirmation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		metadata domain.DocumentMetadata
		valid    bool
	}{
		{"review with unknown dates", domain.DocumentMetadata{Title: "Contract", Status: "review"}, true},
		{"active without dates", domain.DocumentMetadata{Title: "Contract", Status: "active", SignaturesConfirmed: true}, false},
		{"signed without review", domain.DocumentMetadata{Title: "Contract", Status: "signed"}, false},
		{"reversed dates", domain.DocumentMetadata{Title: "Contract", Status: "review", StartDate: "2026-10-16", EndDate: "2026-10-15"}, false},
		{"invalid date", domain.DocumentMetadata{Title: "Contract", Status: "review", SignedDate: "2026-02-30"}, false},
		{"confirmed signatures", domain.DocumentMetadata{Title: "Contract", Status: "signed", SignaturesConfirmed: true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateDocument(tc.metadata); got != tc.valid {
				t.Fatalf("got %v want %v", got, tc.valid)
			}
		})
	}
}
