package httpapi

import (
	"bytes"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) documents(c *gin.Context) repository.DocumentStore {
	s, ok := h.repo.(repository.DocumentStore)
	if !ok {
		writeError(c, 503, "DOCUMENTS_UNAVAILABLE", "Document storage requires PostgreSQL.")
		return nil
	}
	return s
}
func documentError(c *gin.Context, err error) {
	if errors.Is(err, repository.ErrDocumentConflict) {
		writeError(c, 409, "DOCUMENT_CONFLICT", "A new PDF version was uploaded. Reload and review that version before saving or releasing it.")
	} else if errors.Is(err, repository.ErrNotFound) {
		writeError(c, 404, "DOCUMENT_NOT_FOUND", "Document not found.")
	} else if errors.Is(err, repository.ErrSiteEvidenceLimit) {
		writeError(c, 400, "DOCUMENT_LIMIT", "Project documents may total at most 100 MB.")
	} else {
		writeError(c, 500, "DOCUMENT_FAILED", "Unable to process project documents.")
	}
}
func validateDocument(m domain.DocumentMetadata) bool {
	if strings.TrimSpace(m.Title) == "" {
		return false
	}
	for _, date := range []string{m.SignedDate, m.StartDate, m.EndDate} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return false
			}
		}
	}
	if m.StartDate != "" && m.EndDate != "" && m.EndDate < m.StartDate {
		return false
	}
	if (m.Status == "signed" || m.Status == "active") && !m.SignaturesConfirmed {
		return false
	}
	if m.Status == "active" && (m.StartDate == "" || m.EndDate == "" || m.StartDate > time.Now().In(time.FixedZone("Bangkok", 7*3600)).Format("2006-01-02") || m.EndDate < time.Now().In(time.FixedZone("Bangkok", 7*3600)).Format("2006-01-02")) {
		return false
	}
	return true
}
func (h *Handler) ListDocuments(c *gin.Context) {
	_, role, ok := requestUser(c)
	if !ok {
		return
	}
	store := h.documents(c)
	if store == nil {
		return
	}
	var siteID uuid.UUID
	if c.Param("id") != "" {
		siteID, ok = parseID(c)
		if !ok || !h.requireSitePermission(c, siteID, siteRead) {
			return
		}
	}
	all, err := store.ListDocuments(c.Request.Context())
	if err != nil {
		documentError(c, err)
		return
	}
	allowed := map[uuid.UUID]bool{}
	if role != domain.RoleAdmin && role != domain.RoleSuperAdmin {
		user, _, _ := requestUser(c)
		ids, e := h.repo.ListAccessibleSiteIDs(c.Request.Context(), user)
		if e != nil {
			documentError(c, e)
			return
		}
		for _, id := range ids {
			allowed[id] = true
		}
	}
	result := []domain.ProjectDocument{}
	for _, d := range all {
		if siteID != uuid.Nil && d.SiteID != siteID {
			continue
		}
		if role != domain.RoleAdmin && role != domain.RoleSuperAdmin && !allowed[d.SiteID] {
			continue
		}
		if role == domain.RoleCustomer && !d.Metadata.Shared {
			continue
		}
		result = append(result, d)
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"data": result})
}
func (h *Handler) documentForRequest(c *gin.Context, staff bool) (repository.DocumentStore, domain.ProjectDocument, bool) {
	var d domain.ProjectDocument
	id, ok := parseID(c)
	if !ok {
		return nil, d, false
	}
	if staff && !h.requireStaff(c) {
		return nil, d, false
	}
	if !h.requireSitePermission(c, id, siteRead) {
		return nil, d, false
	}
	docID, err := uuid.Parse(c.Param("documentID"))
	if err != nil {
		writeError(c, 400, "INVALID_INPUT", "Invalid document ID.")
		return nil, d, false
	}
	store := h.documents(c)
	if store == nil {
		return nil, d, false
	}
	d, err = store.GetDocument(c.Request.Context(), docID)
	if err != nil {
		documentError(c, err)
		return nil, d, false
	}
	_, role, _ := requestUser(c)
	if d.SiteID != id || (role == domain.RoleCustomer && !d.Metadata.Shared) {
		documentError(c, repository.ErrNotFound)
		return nil, d, false
	}
	return store, d, true
}
func (h *Handler) UploadDocument(c *gin.Context) {
	if !h.requireStaff(c) {
		return
	}
	siteID, ok := parseID(c)
	if !ok || !h.requireSitePermission(c, siteID, siteWrite) {
		return
	}
	store := h.documents(c)
	if store == nil {
		return
	}
	d := domain.ProjectDocument{ID: uuid.New(), SiteID: siteID, Metadata: domain.DocumentMetadata{Category: "contract", Status: "review"}}
	actor, _, _ := requestUser(c)
	if key := c.GetHeader("X-Request-ID"); key != "" && c.Param("documentID") == "" {
		requestID, err := uuid.Parse(key)
		if err != nil {
			writeError(c, 400, "INVALID_INPUT", "X-Request-ID must be a UUID.")
			return
		}
		d.ID = uuid.NewSHA1(actor, []byte("project-document:"+siteID.String()+":"+requestID.String()))
	}
	if c.Param("documentID") != "" {
		var valid bool
		store, d, valid = h.documentForRequest(c, true)
		if !valid {
			return
		}
	} else {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 11<<20)
		d.Metadata.Title = strings.TrimSpace(c.PostForm("title"))
		d.Metadata.Category = c.PostForm("category")
		if len([]rune(d.Metadata.Title)) > 200 || d.Metadata.Title == "" || !strings.Contains("|contract|proposal|plan|handover|", "|"+d.Metadata.Category+"|") {
			writeError(c, 400, "INVALID_INPUT", "Choose a title and document category.")
			return
		}
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 11<<20)
	form, err := c.MultipartForm()
	if err != nil {
		writeError(c, 400, "INVALID_INPUT", "Upload one PDF, no larger than 10 MB.")
		return
	}
	defer form.RemoveAll()
	if len(form.File["file"]) != 1 {
		writeError(c, 400, "INVALID_INPUT", "Upload one PDF.")
		return
	}
	f := form.File["file"][0]
	if f.Size <= 0 || f.Size > 10<<20 {
		writeError(c, 400, "INVALID_INPUT", "PDF must be 10 MB or smaller.")
		return
	}
	opened, err := f.Open()
	if err != nil {
		documentError(c, err)
		return
	}
	defer opened.Close()
	content, err := io.ReadAll(io.LimitReader(opened, (10<<20)+1))
	if err != nil || len(content) > 10<<20 || !bytes.HasPrefix(content, []byte("%PDF-")) || !bytes.Contains(content, []byte("%%EOF")) {
		writeError(c, 400, "INVALID_INPUT", "The file must be a complete PDF.")
		return
	}
	d, err = store.SaveDocument(c.Request.Context(), d, actor, &domain.DocumentVersion{Filename: filepath.Base(f.Filename), Content: content})
	if err != nil {
		documentError(c, err)
		return
	}
	c.JSON(201, gin.H{"data": d})
}
func (h *Handler) UpdateDocument(c *gin.Context) {
	store, d, ok := h.documentForRequest(c, true)
	if !ok {
		return
	}
	if !h.requireSitePermission(c, d.SiteID, siteWrite) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var input struct {
		domain.DocumentMetadata
		ExpectedVersion int `json:"expectedVersion" binding:"required,min=1"`
	}
	if c.ShouldBindJSON(&input) != nil || !validateDocument(input.DocumentMetadata) {
		writeError(c, 400, "INVALID_INPUT", "Check the dates and confirm signatures before marking signed or active.")
		return
	}
	d.Metadata = input.DocumentMetadata
	d.LatestVersion = input.ExpectedVersion
	actor, _, _ := requestUser(c)
	d, err := store.SaveDocument(c.Request.Context(), d, actor, nil)
	if err != nil {
		documentError(c, err)
		return
	}
	c.JSON(200, gin.H{"data": d})
}
func (h *Handler) DocumentVersions(c *gin.Context) {
	store, d, ok := h.documentForRequest(c, false)
	if !ok {
		return
	}
	v, err := store.DocumentVersions(c.Request.Context(), d.ID)
	if err != nil {
		documentError(c, err)
		return
	}
	_, role, _ := requestUser(c)
	if role == domain.RoleCustomer {
		filtered := []domain.DocumentVersion{}
		for _, item := range v {
			if item.Version == d.LatestVersion {
				filtered = append(filtered, item)
			}
		}
		v = filtered
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"data": v})
}
func (h *Handler) DownloadDocument(c *gin.Context) {
	store, d, ok := h.documentForRequest(c, false)
	if !ok {
		return
	}
	version, err := strconv.Atoi(c.Param("version"))
	_, role, _ := requestUser(c)
	if err != nil || version < 1 || (role == domain.RoleCustomer && version != d.LatestVersion) {
		documentError(c, repository.ErrNotFound)
		return
	}
	f, err := store.DocumentFile(c.Request.Context(), d.ID, version)
	if err != nil {
		documentError(c, err)
		return
	}
	actor, _, _ := requestUser(c)
	action, disposition := "previewed", "inline"
	if c.Query("download") == "1" {
		action, disposition = "downloaded", "attachment"
	}
	if err = store.RecordDocumentRead(c.Request.Context(), d.ID, actor, action, version); err != nil {
		documentError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": f.Filename}))
	c.Data(http.StatusOK, "application/pdf", f.Content)
}
func (h *Handler) DocumentEvents(c *gin.Context) {
	store, d, ok := h.documentForRequest(c, true)
	if !ok {
		return
	}
	events, err := store.DocumentEvents(c.Request.Context(), d.ID)
	if err != nil {
		documentError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"data": events})
}
