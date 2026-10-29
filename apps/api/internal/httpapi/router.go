package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rbc/ev-station/apps/api/internal/config"
)

func NewRouter(cfg config.Config, handler *Handler) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	// Log route templates and timing, never credentials, request bodies or
	// query strings. This separates upstream failures from HTTP timeouts.
	router.Use(func(c *gin.Context) {
		started := time.Now()
		c.Next()
		if c.FullPath() == "/health" {
			return
		}
		slog.Info("http request", "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds(), "canceled", c.Request.Context().Err() != nil)
	})
	router.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSAllowedOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization", "X-Request-ID"},
		MaxAge:       12 * time.Hour,
	}))
	router.GET("/health", handler.Health)
	router.GET("/liff/site-submission", handler.LiffPage)
	api := router.Group("/api/v1")
	authLimits := newAuthLimiter()
	api.POST("/auth/register", authLimits.middleware(), handler.Register)
	api.POST("/auth/register/request-otp", authLimits.middleware(), handler.RequestRegistrationOTP)
	api.POST("/auth/login", authLimits.middleware(), handler.Login)
	api.POST("/public/visitor-ordinal", handler.RegisterVisitorOrdinal)
	api.POST("/line/webhook", handler.LineWebhook)
	api.GET("/liff/config", handler.LiffConfig)
	api.GET("/liff/customer-profile", handler.LiffCustomerProfile)
	api.GET("/liff/sites", handler.LiffListSites)
	api.POST("/liff/sites", handler.LiffSubmit)
	api.POST("/liff/sites/:id/complete", handler.LiffComplete)
	api.POST("/liff/maps/resolve", handler.LiffResolveGoogleMapsURL)
	api.POST("/liff/sites/:id/images", handler.LiffImages)
	if cfg.AuthRequired {
		api.Use(handler.RequireAuth)
	}
	api.GET("/users", handler.ListUsers)
	api.POST("/users", handler.CreateUser)
	api.PUT("/users/:id", handler.UpdateUser)
	api.DELETE("/users/:id", handler.DeleteUser)
	api.POST("/sites", handler.CreateSite)
	api.GET("/sites", handler.ListSites)
	api.GET("/documents", handler.ListDocuments)
	api.GET("/document-templates", handler.ListDocumentTemplates)
	api.POST("/document-templates", handler.UploadDocumentTemplate)
	api.GET("/document-templates/:templateID/file", handler.DownloadDocumentTemplate)
	api.GET("/sites/:id/documents", handler.ListDocuments)
	api.POST("/sites/:id/documents", handler.UploadDocument)
	api.PUT("/sites/:id/documents/:documentID", handler.UpdateDocument)
	api.POST("/sites/:id/documents/:documentID/versions", handler.UploadDocument)
	api.GET("/sites/:id/documents/:documentID/versions", handler.DocumentVersions)
	api.GET("/sites/:id/documents/:documentID/versions/:version/file", handler.DownloadDocument)
	api.GET("/sites/:id/documents/:documentID/events", handler.DocumentEvents)
	api.POST("/maps/resolve", handler.ResolveGoogleMapsURL)
	api.GET("/sites/:id/latest-analysis", handler.GetLatestCompletedAnalysisForSite)
	api.GET("/sites/:id", handler.GetSite)
	api.PUT("/sites/:id", handler.UpdateSite)
	api.DELETE("/sites/:id", handler.DeleteSite)
	api.GET("/sites/:id/images", handler.ListSiteImages)
	api.GET("/sites/:id/images/:imageID", handler.GetSiteImage)
	api.POST("/sites/:id/images", handler.UploadSiteImages)
	api.POST("/sites/:id/analyses", handler.RunAnalysis)
	api.GET("/analyses/:id", handler.GetAnalysis)
	api.POST("/analyses/:id/recalculate-preliminary", handler.RecalculatePreliminary)
	api.POST("/analyses/:id/ai-assessment", handler.GenerateAIAssessment)
	api.POST("/analyses/:id/station-recommendation", handler.RecommendStation)
	api.GET("/geocoding/search", handler.SearchAddress)
	api.GET("/data-sources", handler.GetDataSources)
	api.GET("/api-usage", handler.GetAPIUsage)
	api.POST("/api-usage/google-maps-load", handler.RecordGoogleMapsLoad)
	api.POST("/financial/calculate", handler.CalculateFinancial)
	api.GET("/financial/plans", handler.GetFranchisePlans)
	api.GET("/financial/plans/:code", handler.GetFranchisePlan)
	api.GET("/scoring/config", handler.GetScoringConfig)
	return router
}
