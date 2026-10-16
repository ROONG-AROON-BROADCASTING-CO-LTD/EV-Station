package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

type DocumentTemplate = domain.DocumentTemplate

// Templates contain examples with real counterparty details: never public.
type templateStore interface {
	ListDocumentTemplates(context.Context) ([]DocumentTemplate, error)
	SaveDocumentTemplate(context.Context, DocumentTemplate, uuid.UUID) (DocumentTemplate, error)
	ReadDocumentTemplate(context.Context, uuid.UUID, uuid.UUID, string) (DocumentTemplate, error)
}

func (h *Handler) ListDocumentTemplates(c *gin.Context) {
	if !h.requireStaff(c) {
		return
	}
	store, ok := h.repo.(templateStore)
	if !ok {
		writeError(c, 503, "DOCUMENTS_UNAVAILABLE", "Template storage requires PostgreSQL.")
		return
	}
	list, err := store.ListDocumentTemplates(c.Request.Context())
	if err != nil {
		documentError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"data": list})
}
func (h *Handler) UploadDocumentTemplate(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	store, ok := h.repo.(templateStore)
	if !ok {
		writeError(c, 503, "DOCUMENTS_UNAVAILABLE", "Template storage requires PostgreSQL.")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 11<<20)
	form, err := c.MultipartForm()
	if err != nil {
		writeError(c, 400, "INVALID_INPUT", "Upload one PDF up to 10 MB.")
		return
	}
	defer form.RemoveAll()
	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" || len([]rune(title)) > 200 || len(form.File["file"]) != 1 {
		writeError(c, 400, "INVALID_INPUT", "Enter a title and one PDF.")
		return
	}
	file := form.File["file"][0]
	if file.Size <= 0 || file.Size > 10<<20 {
		writeError(c, 400, "INVALID_INPUT", "PDF must be 10 MB or smaller.")
		return
	}
	opened, err := file.Open()
	if err != nil {
		documentError(c, err)
		return
	}
	defer opened.Close()
	data, err := io.ReadAll(io.LimitReader(opened, (10<<20)+1))
	if err != nil || len(data) > 10<<20 || !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data, []byte("%%EOF")) {
		writeError(c, 400, "INVALID_INPUT", "Use a complete PDF up to 10 MB.")
		return
	}
	id := uuid.New()
	actor, _, _ := requestUser(c)
	if key := c.GetHeader("X-Request-ID"); key != "" {
		requestID, e := uuid.Parse(key)
		if e != nil {
			writeError(c, 400, "INVALID_INPUT", "X-Request-ID must be a UUID.")
			return
		}
		id = uuid.NewSHA1(actor, []byte("document-template:"+requestID.String()))
	}
	hash := sha256.Sum256(data)
	result, err := store.SaveDocumentTemplate(c.Request.Context(), DocumentTemplate{ID: id, Title: title, Filename: filepath.Base(file.Filename), Content: data, SHA256: hex.EncodeToString(hash[:])}, actor)
	if err != nil {
		documentError(c, err)
		return
	}
	c.JSON(201, gin.H{"data": result})
}
func (h *Handler) DownloadDocumentTemplate(c *gin.Context) {
	if !h.requireStaff(c) {
		return
	}
	store, ok := h.repo.(templateStore)
	if !ok {
		writeError(c, 503, "DOCUMENTS_UNAVAILABLE", "Template storage requires PostgreSQL.")
		return
	}
	id, err := uuid.Parse(c.Param("templateID"))
	if err != nil {
		writeError(c, 400, "INVALID_INPUT", "Invalid template ID.")
		return
	}
	actor, _, _ := requestUser(c)
	action, disposition := "previewed", "inline"
	if c.Query("download") == "1" {
		action, disposition = "downloaded", "attachment"
	}
	result, err := store.ReadDocumentTemplate(c.Request.Context(), id, actor, action)
	if err != nil {
		documentError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": result.Filename}))
	c.Data(http.StatusOK, "application/pdf", result.Content)
}
