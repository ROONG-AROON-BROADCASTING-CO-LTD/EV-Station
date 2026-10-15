package repository

import (
	"crypto/sha256"
	"fmt"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func evidenceFingerprint(image domain.SiteImage) string {
	return fmt.Sprintf("%s:%x", image.MIMEType, sha256.Sum256(image.Data))
}

// A successful upload whose HTTP response was lost can be submitted again.
// Deduplicate both against stored evidence and within the incoming batch.
func uniqueEvidence(seen map[string]bool, images []domain.SiteImage) []domain.SiteImage {
	result := make([]domain.SiteImage, 0, len(images))
	for _, image := range images {
		key := evidenceFingerprint(image)
		if !seen[key] {
			seen[key] = true
			result = append(result, image)
		}
	}
	return result
}
