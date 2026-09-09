package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/advisory"
	"github.com/rbc/ev-station/apps/api/internal/analysis"
	"github.com/rbc/ev-station/apps/api/internal/auth"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/financial"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/scoring"
	"github.com/rbc/ev-station/apps/api/internal/site"
)

type Handler struct {
	sites    *site.Service
	analyses *analysis.Service
	repo     repository.Repository
	weights  map[string]float64
	geocoder provider.Geocoder
	advisory *advisory.Service
	auth     *auth.Service
	otp      *auth.OTPService
	notifier SiteSubmissionNotifier
}

// SiteSubmissionNotifier delivers a notification after a customer submits a
// site. It is intentionally optional, so the customer submission itself never
// fails simply because a third-party messaging service is unavailable.
type SiteSubmissionNotifier interface {
	NotifyCustomerSubmission(context.Context, domain.Site) error
}

func NewHandler(sites *site.Service, analyses *analysis.Service, repo repository.Repository, geocoder provider.Geocoder, advisoryService *advisory.Service, authService *auth.Service, otpService *auth.OTPService, notifier SiteSubmissionNotifier, weights map[string]float64) *Handler {
	return &Handler{sites: sites, analyses: analyses, repo: repo, geocoder: geocoder, advisory: advisoryService, auth: authService, otp: otpService, notifier: notifier, weights: weights}
}

func (h *Handler) LineWebhook(c *gin.Context) {
	if h.notifier == nil {
		writeError(c, http.StatusServiceUnavailable, "LINE_NOT_CONFIGURED", "LINE webhook is not configured.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_LINE_WEBHOOK", "Unable to read LINE webhook.")
		return
	}
	recorder, ok := h.notifier.(interface {
		RecordWebhook(context.Context, []byte, string) error
	})
	if !ok {
		writeError(c, http.StatusServiceUnavailable, "LINE_NOT_CONFIGURED", "LINE webhook is not configured.")
		return
	}
	if err = recorder.RecordWebhook(c.Request.Context(), body, c.GetHeader("X-Line-Signature")); err != nil {
		writeError(c, http.StatusUnauthorized, "INVALID_LINE_WEBHOOK", "LINE webhook signature is invalid.")
		return
	}
	c.Status(http.StatusOK)
}

func (h *Handler) RequestRegistrationOTP(c *gin.Context) {
	var input struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	if err := h.otp.Request(input.Email); err != nil {
		if errors.Is(err, auth.ErrOTPNotConfigured) {
			writeError(c, http.StatusServiceUnavailable, "OTP_NOT_CONFIGURED", "Email verification is not configured.")
			return
		}
		writeError(c, http.StatusBadGateway, "OTP_DELIVERY_FAILED", "Unable to send verification code.")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"sent": true}})
}

func (h *Handler) Register(c *gin.Context) {
	var input struct {
		Email       string `json:"email" binding:"required,email"`
		DisplayName string `json:"displayName" binding:"required,max=160"`
		Password    string `json:"password" binding:"required,min=8"`
		OTP         string `json:"otp" binding:"required,len=6"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	if !h.otp.Verify(input.Email, input.OTP) {
		writeError(c, http.StatusBadRequest, "OTP_INVALID", "The verification code is invalid or expired.")
		return
	}
	users, err := h.repo.ListUsers(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "USERS_UNAVAILABLE", "Unable to prepare the first owner account.")
		return
	}
	hasOwner := false
	for _, existing := range users {
		if existing.Role == domain.RoleSuperAdmin {
			hasOwner = true
			break
		}
	}
	// Bootstrap exactly one owner. Every later public registration is a customer
	// account, and never accepts a caller-supplied privileged role.
	role := domain.RoleCustomer
	if !hasOwner {
		role = domain.RoleSuperAdmin
	}
	user, err := h.auth.Register(c.Request.Context(), input.Email, input.DisplayName, input.Password, role)
	if err != nil {
		writeError(c, http.StatusConflict, "USER_CREATE_FAILED", "Unable to create this user.")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": user})
}

func (h *Handler) CreateUser(c *gin.Context) {
	_, actorRole, ok := requestUser(c)
	if !ok || actorRole != domain.RoleSuperAdmin {
		if ok {
			writeError(c, http.StatusForbidden, "SUPER_ADMIN_REQUIRED", "Only a super admin can create a team account.")
		}
		return
	}
	var input struct {
		Email       string          `json:"email" binding:"required,email"`
		DisplayName string          `json:"displayName" binding:"required,max=160"`
		Password    string          `json:"password" binding:"required,min=8"`
		Role        domain.UserRole `json:"role" binding:"required,oneof=admin sales"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	user, err := h.auth.Register(c.Request.Context(), input.Email, input.DisplayName, input.Password, input.Role)
	if err != nil {
		writeError(c, http.StatusConflict, "USER_CREATE_FAILED", "Unable to create this user.")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": user})
}
func (h *Handler) RequireAuth(c *gin.Context) {
	value := c.GetHeader("Authorization")
	if !strings.HasPrefix(value, "Bearer ") {
		writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Please sign in to continue.")
		c.Abort()
		return
	}
	claims, err := h.auth.Verify(strings.TrimPrefix(value, "Bearer "))
	if err != nil {
		writeError(c, http.StatusUnauthorized, "INVALID_TOKEN", "Your session has expired. Please sign in again.")
		c.Abort()
		return
	}
	c.Set("userId", claims.Subject)
	c.Set("role", claims.Role)
	c.Next()
}
func (h *Handler) Login(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	user, token, err := h.auth.Login(c.Request.Context(), input.Email, input.Password)
	if err != nil {
		writeError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email or password is incorrect.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"user": user, "token": token}})
}
func (h *Handler) ListUsers(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	users, err := h.repo.ListUsers(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "USERS_UNAVAILABLE", "Unable to load users.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users})
}
func (h *Handler) SetSiteAccess(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	siteID, ok := parseID(c)
	if !ok {
		return
	}
	var input struct {
		UserID uuid.UUID `json:"userId" binding:"required"`
		Role   string    `json:"role" binding:"required,oneof=sales customer"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	if err := h.repo.SetSiteAccess(c.Request.Context(), domain.SiteAccess{SiteID: siteID, UserID: input.UserID, Role: input.Role}); err != nil {
		writeError(c, http.StatusNotFound, "SITE_OR_USER_NOT_FOUND", "Site or user was not found.")
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) DeleteSiteAccess(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	siteID, ok := parseID(c)
	if !ok {
		return
	}
	userID, err := uuid.Parse(c.Param("userID"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_USER_ID", "User ID is invalid.")
		return
	}
	if err := h.repo.DeleteSiteAccess(c.Request.Context(), siteID, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(c, http.StatusNotFound, "SITE_ACCESS_NOT_FOUND", "This site access assignment was not found.")
			return
		}
		writeError(c, http.StatusInternalServerError, "SITE_ACCESS_DELETE_FAILED", "Unable to remove site access.")
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) ListSiteAccess(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}
	siteID, ok := parseID(c)
	if !ok {
		return
	}
	access, err := h.repo.ListSiteAccess(c.Request.Context(), siteID)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "Site not found.")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "ACCESS_UNAVAILABLE", "Unable to load access.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": access})
}

const (
	maxSiteImages     = 10
	maxSiteImageBytes = 10 << 20
	maxSitePDFBytes   = 10 << 20
	// Allow up to ten 10 MB documents plus multipart form overhead.
	maxUploadBytes = 102 << 20
)

func (h *Handler) SearchAddress(c *gin.Context) {
	query := c.Query("q")
	results, err := h.geocoder.Search(c.Request.Context(), query, parseLimit(c.Query("limit"), 5))
	if errors.Is(err, provider.ErrInvalidGeocodingQuery) {
		writeError(c, http.StatusBadRequest, "INVALID_GEOCODING_QUERY", err.Error())
		return
	}
	if err != nil {
		writeError(c, http.StatusBadGateway, "GEOCODING_UNAVAILABLE", "The free geocoding provider is temporarily unavailable.")
		return
	}
	if results == nil {
		results = []provider.GeocodingResult{}
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

func (h *Handler) GetDataSources(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": provider.DataSourceCatalog()})
}

func (h *Handler) Health(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) }

func (h *Handler) ResolveGoogleMapsURL(c *gin.Context) {
	var input struct {
		URL string `json:"url" binding:"required,max=2000"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_GOOGLE_MAPS_URL", "Google Maps link is required.")
		return
	}
	result, err := provider.ResolveGoogleMapsURL(c.Request.Context(), input.URL, h.geocoder)
	if errors.Is(err, provider.ErrInvalidGoogleMapsURL) {
		writeError(c, http.StatusBadRequest, "INVALID_GOOGLE_MAPS_URL", "Only HTTPS Google Maps links are supported.")
		return
	}
	if errors.Is(err, provider.ErrGoogleMapsCoordinatesNotFound) {
		writeError(c, http.StatusUnprocessableEntity, "GOOGLE_MAPS_COORDINATES_NOT_FOUND", "Coordinates were not found in the Google Maps destination link.")
		return
	}
	if err != nil {
		writeError(c, http.StatusBadGateway, "GOOGLE_MAPS_UNAVAILABLE", "Google Maps link could not be resolved right now.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) CreateSite(c *gin.Context) {
	userID, role, ok := requestUser(c)
	if !ok {
		return
	}
	var input domain.CreateSiteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.sites.Create(c.Request.Context(), input)
	if errors.Is(err, site.ErrInvalidLocation) {
		writeError(c, http.StatusBadRequest, "INVALID_LOCATION", err.Error())
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "SITE_CREATE_FAILED", "Unable to save the site.")
		return
	}
	if role == domain.RoleSales || role == domain.RoleCustomer {
		if err := h.repo.SetSiteAccess(c.Request.Context(), domain.SiteAccess{SiteID: result.ID, UserID: userID, Role: string(role)}); err != nil {
			writeError(c, http.StatusInternalServerError, "SITE_ACCESS_SAVE_FAILED", "Site was created but could not be assigned to the sales user.")
			return
		}
	}
	if role == domain.RoleCustomer && h.notifier != nil {
		go func(site domain.Site) {
			if notifyErr := h.notifier.NotifyCustomerSubmission(context.Background(), site); notifyErr != nil {
				slog.Error("customer site LINE notification failed", "site_id", site.ID, "error", notifyErr)
			}
		}(result)
	}
	c.JSON(http.StatusCreated, gin.H{"data": result})
}

func (h *Handler) ListSites(c *gin.Context) {
	result, err := h.sites.List(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "SITE_LIST_FAILED", "Unable to load sites.")
		return
	}
	if result == nil {
		result = []domain.Site{}
	}
	userID, role, ok := requestUser(c)
	if !ok {
		return
	}
	if role != domain.RoleSuperAdmin && role != domain.RoleAdmin {
		allowed := make([]domain.Site, 0, len(result))
		for _, candidate := range result {
			access, accessErr := h.repo.ListSiteAccess(c.Request.Context(), candidate.ID)
			if accessErr != nil {
				continue
			}
			for _, grant := range access {
				if grant.UserID == userID {
					allowed = append(allowed, candidate)
					break
				}
			}
		}
		result = allowed
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) GetSite(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if !h.requireSitePermission(c, id, siteRead) {
		return
	}
	result, err := h.sites.Get(c.Request.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "Site not found.")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "SITE_LOAD_FAILED", "Unable to load the site.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) UploadSiteImages(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if !h.requireSitePermission(c, id, siteWrite) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)
	form, err := c.MultipartForm()
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_SITE_IMAGES", "Upload up to 10 JPEG, PNG, WebP, or PDF files with a maximum size of 10 MB each.")
		return
	}
	files := form.File["images"]
	if len(files) == 0 || len(files) > maxSiteImages {
		writeError(c, http.StatusBadRequest, "INVALID_SITE_IMAGES", "Upload between 1 and 10 images.")
		return
	}
	existing, err := h.repo.GetSiteImages(c.Request.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "Site not found.")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "SITE_IMAGE_LOAD_FAILED", "Unable to prepare site image upload.")
		return
	}
	if len(existing)+len(files) > maxSiteImages {
		writeError(c, http.StatusBadRequest, "INVALID_SITE_IMAGES", "A site can have no more than 10 images. Remove existing images or upload fewer images.")
		return
	}
	images := make([]domain.SiteImage, 0, len(files))
	for _, file := range files {
		if file.Size <= 0 || file.Size > maxSitePDFBytes {
			writeError(c, http.StatusBadRequest, "INVALID_SITE_IMAGES", "Each file must be no larger than 10 MB.")
			return
		}
		opened, openErr := file.Open()
		if openErr != nil {
			writeError(c, http.StatusBadRequest, "INVALID_SITE_IMAGES", "An uploaded image could not be read.")
			return
		}
		data, readErr := io.ReadAll(io.LimitReader(opened, maxSitePDFBytes+1))
		_ = opened.Close()
		if readErr != nil || len(data) == 0 || len(data) > maxSitePDFBytes {
			writeError(c, http.StatusBadRequest, "INVALID_SITE_IMAGES", "An uploaded image could not be read.")
			return
		}
		mimeType := http.DetectContentType(data)
		if mimeType != "image/jpeg" && mimeType != "image/png" && mimeType != "image/webp" && mimeType != "application/pdf" {
			writeError(c, http.StatusBadRequest, "INVALID_SITE_IMAGES", "Only JPEG, PNG, WebP, and PDF files are supported.")
			return
		}
		maxBytes := maxSiteImageBytes
		if mimeType == "application/pdf" {
			maxBytes = maxSitePDFBytes
		}
		if len(data) > maxBytes {
			writeError(c, http.StatusBadRequest, "INVALID_SITE_IMAGES", "Each file must be no larger than 10 MB.")
			return
		}
		images = append(images, domain.SiteImage{ID: uuid.New(), SiteID: id, MIMEType: mimeType, Data: data, CreatedAt: time.Now().UTC()})
	}
	if err = h.repo.AddSiteImages(c.Request.Context(), id, images); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "Site not found.")
			return
		}
		writeError(c, http.StatusInternalServerError, "SITE_IMAGE_UPLOAD_FAILED", "Unable to save site evidence files.")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"count": len(images)}})
}

type siteImageResponse struct {
	ID        uuid.UUID `json:"id"`
	MIMEType  string    `json:"mimeType"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt"`
}

func (h *Handler) ListSiteImages(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if !h.requireSitePermission(c, id, siteRead) {
		return
	}
	images, err := h.repo.ListSiteImages(c.Request.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "Site not found.")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "SITE_IMAGE_LOAD_FAILED", "Unable to load site evidence files.")
		return
	}
	response := make([]siteImageResponse, 0, len(images))
	for _, image := range images {
		response = append(response, siteImageResponse{ID: image.ID, MIMEType: image.MIMEType, SizeBytes: image.SizeBytes, CreatedAt: image.CreatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"data": response})
}

func (h *Handler) GetSiteImage(c *gin.Context) {
	siteID, ok := parseID(c)
	if !ok {
		return
	}
	if !h.requireSitePermission(c, siteID, siteRead) {
		return
	}
	imageID, err := uuid.Parse(c.Param("imageID"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_SITE_IMAGE", "Invalid site evidence file ID.")
		return
	}
	image, err := h.repo.GetSiteImage(c.Request.Context(), siteID, imageID)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "SITE_IMAGE_NOT_FOUND", "Site evidence file not found.")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "SITE_IMAGE_LOAD_FAILED", "Unable to load site evidence file.")
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "inline")
	c.Data(http.StatusOK, image.MIMEType, image.Data)
}

func (h *Handler) UpdateSite(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if !h.requireSitePermission(c, id, siteWrite) {
		return
	}
	var input domain.CreateSiteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.sites.Update(c.Request.Context(), id, input)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "Site not found.")
		return
	}
	if errors.Is(err, site.ErrInvalidLocation) {
		writeError(c, http.StatusBadRequest, "INVALID_LOCATION", err.Error())
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "SITE_UPDATE_FAILED", "Unable to update the site.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) DeleteSite(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if !h.requireAdmin(c) {
		return
	}
	if err := h.sites.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "Site not found.")
			return
		}
		writeError(c, http.StatusInternalServerError, "SITE_DELETE_FAILED", "Unable to delete the site.")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) GetLatestCompletedAnalysisForSite(c *gin.Context) {
	siteID, ok := parseID(c)
	if !ok {
		return
	}
	if !h.requireSitePermission(c, siteID, siteRead) {
		return
	}
	result, err := h.analyses.GetLatestCompletedForSite(c.Request.Context(), siteID)
	if errors.Is(err, repository.ErrNotFound) {
		// No previous completed analysis is a normal state for a newly created site.
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "ANALYSIS_LOAD_FAILED", "Unable to load the latest analysis.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) RunAnalysis(c *gin.Context) {
	if !h.requireStaff(c) {
		return
	}
	siteID, ok := parseID(c)
	if !ok {
		return
	}
	if !h.requireSitePermission(c, siteID, siteWrite) {
		return
	}
	var body struct {
		RadiusMeters int `json:"radiusMeters"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
	}
	if body.RadiusMeters != 0 && body.RadiusMeters != 1000 && body.RadiusMeters != 2000 && body.RadiusMeters != 3000 {
		writeError(c, http.StatusBadRequest, "INVALID_RADIUS", "Radius must be 1000, 2000, or 3000 meters.")
		return
	}
	result, err := h.analyses.Run(c.Request.Context(), siteID, body.RadiusMeters)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "SITE_NOT_FOUND", "Site not found.")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "ANALYSIS_FAILED", "Unable to complete the analysis.")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": result})
}

func (h *Handler) GetAnalysis(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if _, ok := h.requireAnalysisPermission(c, id, siteRead); !ok {
		return
	}
	result, err := h.analyses.Get(c.Request.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "ANALYSIS_NOT_FOUND", "Analysis not found.")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "ANALYSIS_LOAD_FAILED", "Unable to load the analysis.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) RecalculatePreliminary(c *gin.Context) {
	if !h.requireStaff(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if _, ok := h.requireAnalysisPermission(c, id, siteWrite); !ok {
		return
	}
	result, err := h.analyses.RecalculatePreliminary(c.Request.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(c, http.StatusNotFound, "ANALYSIS_NOT_FOUND", "Analysis not found.")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "SCORING_UPDATE_FAILED", "Unable to calculate the preliminary screening score.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) GenerateAIAssessment(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	run, ok := h.requireAnalysisPermission(c, id, siteRead)
	if !ok {
		return
	}
	var body struct {
		Language string `json:"language"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			writeError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
	}
	if body.Language != "" && body.Language != "th" && body.Language != "en" {
		writeError(c, http.StatusBadRequest, "INVALID_LANGUAGE", "Language must be th or en.")
		return
	}
	language := body.Language
	if language == "" {
		language = "th"
	}
	refresh := c.Query("refresh") == "true"
	expectedDecision := domain.InvestmentDecisionForScore(run.OverallScore)
	var cached map[string]domain.AIAssessment
	if !refresh && len(run.AIAssessments) > 0 {
		if err := json.Unmarshal(run.AIAssessments, &cached); err == nil {
			if assessment, ok := cached[language]; ok && assessment.Language == language && assessment.Decision == expectedDecision && assessment.DecisionPolicy == domain.InvestmentDecisionPolicyVersion {
				c.JSON(http.StatusOK, gin.H{"data": assessment})
				return
			}
		}
	}
	if !h.requireStaff(c) {
		return
	}
	if !h.requireSitePermission(c, run.SiteID, siteWrite) {
		return
	}
	if h.advisory == nil {
		writeError(c, http.StatusServiceUnavailable, "AI_NOT_CONFIGURED", "AI assessment is not configured.")
		return
	}
	result, err := h.advisory.Generate(c.Request.Context(), run, language)
	if errors.Is(err, advisory.ErrNotConfigured) {
		writeError(c, http.StatusServiceUnavailable, "AI_NOT_CONFIGURED", "Set GEMINI_API_KEY on the API server before generating an AI assessment.")
		return
	}
	if errors.Is(err, advisory.ErrUnavailable) || errors.Is(err, advisory.ErrInvalidOutput) {
		// A completed screening must remain readable even if the optional AI
		// narrative is unavailable or returns malformed structured content.
		result = advisory.FallbackAssessment(run, language)
		err = nil
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "AI_ASSESSMENT_FAILED", "Unable to generate the AI assessment.")
		return
	}
	if cached == nil {
		cached = make(map[string]domain.AIAssessment)
	}
	cached[language] = result
	persisted, err := json.Marshal(cached)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "AI_ASSESSMENT_ENCODE_FAILED", "Unable to save the AI assessment.")
		return
	}
	if err := h.repo.UpdateAnalysisAIAssessments(c.Request.Context(), run.ID, persisted); err != nil {
		writeError(c, http.StatusInternalServerError, "AI_ASSESSMENT_SAVE_FAILED", "Unable to save the AI assessment.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) CalculateFinancial(c *gin.Context) {
	var input financial.Input
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_FINANCIAL_INPUT", err.Error())
		return
	}
	result, err := financial.Calculate(input)
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_FINANCIAL_INPUT", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *Handler) GetFranchisePlans(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": financial.FranchisePlans()})
}

func (h *Handler) GetFranchisePlan(c *gin.Context) {
	plan, err := financial.GetFranchisePlan(c.Param("code"))
	if errors.Is(err, financial.ErrUnknownPlan) {
		writeError(c, http.StatusNotFound, "FRANCHISE_PLAN_NOT_FOUND", "Franchise plan not found.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plan})
}

func (h *Handler) GetScoringConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"weights": h.weights, "status": "provisional", "version": scoring.PreliminaryVersion, "minimumCoveragePercentage": scoring.MinimumCoveragePercentage, "total": 1}})
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_ID", "The supplied id is invalid.")
		return uuid.Nil, false
	}
	return id, true
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func parseLimit(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

var _ = scoring.DefaultWeights
