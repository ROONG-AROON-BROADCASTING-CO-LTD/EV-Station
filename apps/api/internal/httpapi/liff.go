package httpapi

import (
	"context"
	_ "embed"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/site"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed liff.html
var liffHTML []byte

type lineIdentity interface {
	VerifyIDToken(context.Context, string, string) (string, error)
	ConfirmSubmission(context.Context, string, domain.Site) error
}
type lineStore interface {
	SaveLineSubmission(context.Context, string, uuid.UUID, domain.Site) (domain.Site, error)
	OwnsLineSite(context.Context, string, uuid.UUID) (bool, error)
	ListLineSites(context.Context, string) ([]domain.Site, error)
	GetLineCustomerProfile(context.Context, string) (domain.LineCustomerProfile, error)
	UpsertLineCustomerProfile(context.Context, string, domain.LineCustomerProfile) (domain.LineCustomerProfile, error)
}

func (h *Handler) LiffPage(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Data(200, "text/html; charset=utf-8", liffHTML)
}
func (h *Handler) LiffConfig(c *gin.Context) {
	c.JSON(200, gin.H{"liffId": os.Getenv("LINE_LIFF_ID")})
}

// LiffCustomerProfile returns only the verified caller's saved contact details.
// A missing profile is normal for a customer's first submission.
func (h *Handler) LiffCustomerProfile(c *gin.Context) {
	user, ok := h.lineUser(c)
	if !ok {
		return
	}
	store, ok := h.repo.(lineStore)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "ระบบยังไม่พร้อม")
		return
	}
	profile, err := store.GetLineCustomerProfile(c.Request.Context(), user)
	if err == repository.ErrNotFound {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"profile": nil}})
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "PROFILE_LOAD_FAILED", "ไม่สามารถโหลดข้อมูลผู้ติดต่อได้")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"profile": profile}})
}
func (h *Handler) lineUser(c *gin.Context) (string, bool) {
	n, ok := h.notifier.(lineIdentity)
	if !ok || os.Getenv("LINE_LOGIN_CHANNEL_ID") == "" {
		writeError(c, 503, "LINE_UNAVAILABLE", "ระบบยังไม่พร้อม กรุณาลองใหม่ภายหลัง")
		return "", false
	}
	token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	user, err := n.VerifyIDToken(c.Request.Context(), token, os.Getenv("LINE_LOGIN_CHANNEL_ID"))
	if err != nil {
		writeError(c, 401, "LINE_AUTH_FAILED", "กรุณาเปิดฟอร์มใหม่จาก LINE เพื่อยืนยันตัวตน")
		return "", false
	}
	return user, true
}
func (h *Handler) LiffSubmit(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	user, ok := h.lineUser(c)
	if !ok {
		return
	}
	store, ok := h.repo.(lineStore)
	if !ok {
		writeError(c, 503, "STORAGE_UNAVAILABLE", "ระบบยังไม่พร้อม")
		return
	}
	var in struct {
		domain.CreateSiteInput
		RequestID string `json:"requestId" binding:"required,uuid"`
	}
	if c.ShouldBindJSON(&in) != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.ContactName) == "" || strings.TrimSpace(in.ContactPhone) == "" {
		writeError(c, 400, "INVALID_INPUT", "กรุณากรอกชื่อพื้นที่ ชื่อผู้ติดต่อ เบอร์โทร และขนาดพื้นที่ให้ครบ")
		return
	}
	// The customer-facing form accepts a Google Maps link. Resolve it on the
	// server, after LINE identity is verified, so the same validated coordinates
	// are used by the back-office analysis workflow.
	if in.Latitude == nil && in.Longitude == nil && strings.TrimSpace(in.GoogleMapsURL) != "" {
		resolved, resolveErr := provider.ResolveGoogleMapsURL(c.Request.Context(), in.GoogleMapsURL, h.geocoder)
		if resolveErr == nil {
			in.Latitude = &resolved.Latitude
			in.Longitude = &resolved.Longitude
		} else if strings.TrimSpace(in.Address) == "" && provider.IsGoogleMapsURL(in.GoogleMapsURL) {
			// A valid Maps short link can identify a place without publishing its
			// coordinates to unauthenticated requests. Preserve the customer's
			// location reference and let staff verify the pin from the link.
			in.Address = "ตำแหน่งตามลิงก์ Google Maps (รอตรวจสอบพิกัด)"
		}
	}
	if site.ValidateLocation(in.Address, in.Latitude, in.Longitude) != nil {
		writeError(c, 400, "INVALID_LOCATION", "กรุณาระบุที่อยู่ หรือพิกัดพื้นที่ให้ครบ")
		return
	}
	now := time.Now().UTC()
	if _, err := store.UpsertLineCustomerProfile(c.Request.Context(), user, domain.LineCustomerProfile{ContactName: strings.TrimSpace(in.ContactName), ContactPhone: strings.TrimSpace(in.ContactPhone), UpdatedAt: now}); err != nil {
		writeError(c, http.StatusInternalServerError, "PROFILE_SAVE_FAILED", "ไม่สามารถบันทึกข้อมูลผู้ติดต่อได้")
		return
	}
	s := domain.Site{ID: uuid.New(), Name: strings.TrimSpace(in.Name), ContactName: strings.TrimSpace(in.ContactName), ContactPhone: strings.TrimSpace(in.ContactPhone), Address: in.Address, Latitude: in.Latitude, Longitude: in.Longitude, GoogleMapsURL: in.GoogleMapsURL, LandSize: in.LandSize, LandSizeUnit: in.LandSizeUnit, InternetAvailable: in.InternetAvailable, FrontageMeters: in.FrontageMeters, Notes: in.Notes, InputStatus: domain.DataPreliminary, CreatedAt: now, UpdatedAt: now}
	s, err := store.SaveLineSubmission(c.Request.Context(), user, uuid.MustParse(in.RequestID), s)
	if err != nil {
		writeError(c, 500, "SAVE_FAILED", "บันทึกไม่สำเร็จ กรุณาลองส่งอีกครั้ง")
		return
	}
	// Confirmation is sent by LiffComplete after the optional evidence files
	// have been uploaded. Sending it here used to confirm a submission before a
	// later file-upload failure was known to the customer.
	c.JSON(201, gin.H{"data": gin.H{"id": s.ID, "site": s}})
}

// LiffComplete sends the confirmation only after the client has uploaded all
// selected evidence files. LINE's retry key is the site ID, making repeats
// safe when the mobile network drops after a successful push.
func (h *Handler) LiffComplete(c *gin.Context) {
	user, ok := h.lineUser(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_SITE_ID", "ไม่พบข้อมูลพื้นที่")
		return
	}
	store, ok := h.repo.(lineStore)
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "ระบบยังไม่พร้อม")
		return
	}
	owns, err := store.OwnsLineSite(c.Request.Context(), user, id)
	if err != nil || !owns {
		writeError(c, http.StatusForbidden, "SITE_ACCESS_DENIED", "ไม่สามารถยืนยันข้อมูลพื้นที่นี้ได้")
		return
	}
	s, err := h.repo.GetSite(c.Request.Context(), id)
	if err != nil {
		writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "ไม่พบข้อมูลพื้นที่")
		return
	}
	err = h.notifier.(lineIdentity).ConfirmSubmission(c.Request.Context(), user, s)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"notificationAccepted": err == nil}})
}

// LiffListSites lets a customer see only submissions owned by their verified
// LINE identity. It deliberately does not use the staff dashboard session.
func (h *Handler) LiffListSites(c *gin.Context) {
	user, ok := h.lineUser(c)
	if !ok {
		return
	}
	store, ok := h.repo.(lineStore)
	if !ok {
		writeError(c, 503, "STORAGE_UNAVAILABLE", "ระบบยังไม่พร้อม")
		return
	}
	sites, err := store.ListLineSites(c.Request.Context(), user)
	if err != nil {
		writeError(c, 500, "SITE_LIST_FAILED", "ไม่สามารถโหลดข้อมูลพื้นที่ได้")
		return
	}
	type siteSummary struct {
		domain.Site
		CustomerStatus string `json:"customerStatus"`
	}
	result := make([]siteSummary, 0, len(sites))
	for _, s := range sites {
		status := "pending_review"
		if _, analysisErr := h.repo.GetLatestCompletedAnalysisForSite(c.Request.Context(), s.ID); analysisErr == nil {
			status = "analysis_completed"
		}
		result = append(result, siteSummary{Site: s, CustomerStatus: status})
	}
	c.JSON(200, gin.H{"data": result})
}

// LiffResolveGoogleMapsURL resolves only after the caller proved their LINE
// identity. This avoids exposing a general-purpose map resolver as a public
// unauthenticated endpoint while retaining the existing customer form UX.
func (h *Handler) LiffResolveGoogleMapsURL(c *gin.Context) {
	if _, ok := h.lineUser(c); !ok {
		return
	}
	var input struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_GOOGLE_MAPS_URL", "กรุณาใส่ลิงก์ Google Maps")
		return
	}
	result, err := provider.ResolveGoogleMapsURL(c.Request.Context(), input.URL, h.geocoder)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_GOOGLE_MAPS_URL", "ไม่สามารถอ่านพิกัดจากลิงก์ Google Maps นี้ได้")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) LiffImages(c *gin.Context) {
	user, ok := h.lineUser(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Status(400)
		return
	}
	store, ok := h.repo.(lineStore)
	if !ok {
		c.Status(503)
		return
	}
	owns, err := store.OwnsLineSite(c.Request.Context(), user, id)
	if err != nil || !owns {
		c.Status(403)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32*1024*1024)
	form, err := c.MultipartForm()
	if err != nil {
		writeError(c, 400, "INVALID_FILES", "ไฟล์มีขนาดใหญ่เกินไป")
		return
	}
	defer form.RemoveAll()
	files := form.File["images"]
	if len(files) == 0 || len(files) > 10 {
		c.Status(400)
		return
	}
	// Use the existing upload validation and repository path after verified ownership.
	c.Set("userId", uuid.Nil.String())
	c.Set("role", domain.RoleAdmin)
	h.UploadSiteImages(c)
}
