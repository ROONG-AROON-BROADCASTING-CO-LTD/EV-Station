package domain

import (
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

type DocumentMetadata struct {
	Title               string `json:"title" binding:"required,max=200"`
	Category            string `json:"category" binding:"required,oneof=contract proposal plan handover"`
	ContractNumber      string `json:"contractNumber" binding:"max=100"`
	Counterparty        string `json:"counterparty" binding:"max=200"`
	SignedDate          string `json:"signedDate" binding:"max=10"`
	StartDate           string `json:"startDate" binding:"max=10"`
	EndDate             string `json:"endDate" binding:"max=10"`
	Responsible         string `json:"responsible" binding:"max=160"`
	Summary             string `json:"summary" binding:"max=5000"`
	Status              string `json:"status" binding:"required,oneof=draft review awaiting_signature signed active expired cancelled"`
	SignaturesConfirmed bool   `json:"signaturesConfirmed"`
	Shared              bool   `json:"shared"`
}
type ProjectDocument struct {
	ID            uuid.UUID        `json:"id"`
	SiteID        uuid.UUID        `json:"siteId"`
	SiteName      string           `json:"siteName"`
	Metadata      DocumentMetadata `json:"metadata"`
	LatestVersion int              `json:"latestVersion"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
}
type DocumentVersion struct {
	Version   int       `json:"version"`
	Filename  string    `json:"filename"`
	Size      int       `json:"size"`
	SHA256    string    `json:"sha256"`
	ActorID   uuid.UUID `json:"actorId"`
	CreatedAt time.Time `json:"createdAt"`
	Content   []byte    `json:"-"`
}
type DocumentEvent struct {
	ID        int64           `json:"id"`
	ActorID   uuid.UUID       `json:"actorId"`
	ActorName string          `json:"actorName"`
	Detail    json.RawMessage `json:"detail"`
	Action    string          `json:"action"`
	CreatedAt time.Time       `json:"createdAt"`
}

type DocumentTemplate struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Filename  string    `json:"filename"`
	Size      int       `json:"size"`
	SHA256    string    `json:"sha256"`
	CreatedAt time.Time `json:"createdAt"`
	Content   []byte    `json:"-"`
}
