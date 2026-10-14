package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/advisory"
	"github.com/rbc/ev-station/apps/api/internal/analysis"
	"github.com/rbc/ev-station/apps/api/internal/auth"
	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/config"
	"github.com/rbc/ev-station/apps/api/internal/httpapi"
	lineapi "github.com/rbc/ev-station/apps/api/internal/line"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/roads"
	"github.com/rbc/ev-station/apps/api/internal/scoring"
	"github.com/rbc/ev-station/apps/api/internal/site"
	"github.com/rbc/ev-station/apps/api/internal/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid application configuration", "error", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	telemetryRecorder := telemetry.NewSlogRecorder(logger)
	providerClientWithTimeout := func(providerName string, timeout time.Duration) *http.Client {
		return telemetry.NewHTTPClient(providerName, timeout, telemetryRecorder)
	}
	providerClient := func(providerName string) *http.Client {
		return providerClientWithTimeout(providerName, cfg.ExternalHTTPTimeout)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var externalCache cache.Cache = cache.Noop{}
	if strings.TrimSpace(cfg.RedisURL) != "" {
		redisCache, cacheErr := cache.NewRedis(cfg.RedisURL)
		if cacheErr != nil {
			logger.Warn("redis configuration is invalid; cache disabled", "error", cacheErr)
		} else {
			defer redisCache.Close()
			if !redisCache.Available(ctx) {
				logger.Warn("redis unavailable; cache disabled for this run")
			} else {
				externalCache = redisCache
			}
		}
	}
	providerCache := func(providerName string) cache.Cache {
		return telemetry.WrapCache(externalCache, providerName, telemetryRecorder)
	}
	geocoder := provider.NewNominatimGeocoder(provider.NominatimConfig{
		Endpoint: cfg.NominatimURL, UserAgent: cfg.ExternalUserAgent,
		CountryCodes: cfg.NominatimCountryCodes, CacheTTL: cfg.GeocodingCacheTTL,
	}, providerClient("nominatim"), providerCache("nominatim"))

	analysisGeocoder := provider.NewSharedReverseGeocoder(geocoder)

	var repo repository.Repository
	var localRoads roads.Lookup
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		logger.Warn("DATABASE_URL is not set; using non-persistent in-memory repository")
		repo = repository.NewMemory()
	} else {
		postgresRepo, err := repository.NewPostgres(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("database connection failed", "error", err)
			os.Exit(1)
		}
		defer postgresRepo.Close()
		repo = postgresRepo
		localRoads = postgresRepo.Roads()
	}

	var dataProvider provider.AnalysisProvider = provider.UnavailableProvider{}
	recordUsage := func(requestCtx context.Context, providerID string, units int64) {
		if usageErr := repo.RecordAPIUsage(requestCtx, providerID, units); usageErr != nil {
			logger.Warn("could not record API usage", "provider", providerID, "error", usageErr)
		}
	}
	if cfg.AnalysisProviderMode == "fixture" && cfg.Environment != "production" {
		logger.Warn("using deterministic development fixture provider; results are not factual")
		dataProvider = provider.FixtureProvider{}
	} else if cfg.AnalysisProviderMode == "osm" {
		logger.Info("using Google Places, Open Charge Map, OpenStreetMap, provincial charger, WorldPop, DDPM flood history, GISTDA elevation, DOH/DRR AADT, DLT, MEA and PEA Power Map providers")
		// Road geometry and mapped chargers change less often than the default
		// 15-minute cache window. Reuse a successful Overpass lookup for an hour
		// to avoid repeating the slowest public request for the same site.
		osmCacheTTL := cfg.RedisCacheTTL
		if osmCacheTTL < time.Hour {
			osmCacheTTL = time.Hour
		}
		osmProvider := provider.NewOSMProvider(provider.OSMConfig{
			LocalRoads: localRoads,
			Endpoint:   cfg.OverpassURL, FallbackEndpoints: cfg.OverpassFallbackURLs,
			UserAgent: cfg.ExternalUserAgent, CacheTTL: osmCacheTTL,
			// Each mirror gets up to 10 seconds within a 30-second total budget.
			// The prior 4/8-second limits discarded valid urban-road responses
			// and could expire before the third mirror was attempted. The job
			// runs asynchronously within the 35-second composite deadline.
		}, providerClientWithTimeout("osm", 12*time.Second), providerCache("osm"))
		worldPopProvider := provider.NewWorldPopProvider(provider.WorldPopConfig{
			Endpoint: cfg.WorldPopURL, Year: cfg.WorldPopYear, Resolution: cfg.WorldPopResolution,
			CacheTTL: cfg.WorldPopCacheTTL, UserAgent: cfg.ExternalUserAgent,
		}, providerClient("worldpop"), providerCache("worldpop"))
		dpmFloodHistoryProvider := provider.NewDPMFloodHistoryProvider(provider.DPMFloodHistoryConfig{
			CSVURLs: cfg.DPMFloodHistoryCSVURLs, CacheTTL: cfg.DPMFloodHistoryCacheTTL,
			UserAgent: cfg.ExternalUserAgent,
		}, providerClient("dpm_flood_history"), providerCache("dpm_flood_history"), analysisGeocoder)
		gistdaElevationProvider := provider.NewGISTDAElevationProvider(provider.GISTDAElevationConfig{
			Endpoint: cfg.GISTDAElevationURL, APIKey: cfg.GISTDAAPIKey, CacheTTL: cfg.GISTDAElevationCacheTTL,
			UserAgent: cfg.ExternalUserAgent, UsageRecorder: recordUsage,
		}, providerClient("gistda_elevation"), providerCache("gistda_elevation"))
		gistdaRecentFloodProvider := provider.NewGISTDARecentFloodProvider(provider.GISTDARecentFloodConfig{
			BaseURL: cfg.GISTDADisasterBaseURL, APIKey: cfg.GISTDADisasterAPIKey, UserAgent: cfg.ExternalUserAgent,
		}, providerClientWithTimeout("gistda_recent_flood", 8*time.Second), providerCache("gistda_recent_flood"))
		gistdaGatewayFloodProvider := provider.NewGISTDAGatewayFloodProvider(provider.GISTDAGatewayFloodConfig{
			BaseURL: cfg.GISTDAGIServiceBaseURL, APIKey: cfg.GISTDAGIServiceAPIKey, UserAgent: cfg.ExternalUserAgent,
			Geocoder: analysisGeocoder,
		}, providerClientWithTimeout("gistda_gateway_flood", 18*time.Second), providerCache("gistda_gateway_flood"))
		dohAADTProvider := provider.NewDOHAADTProvider(provider.DOHAADTConfig{
			CSVURL: cfg.DOHAADTCSVURL, RoadLayerURL: cfg.DOHAADTRoadLayerURL, DataYear: cfg.DOHAADTYear,
			CacheTTL: cfg.DOHAADTCacheTTL, UserAgent: cfg.ExternalUserAgent,
		}, providerClient("doh_aadt"), providerCache("doh_aadt"))
		drrAADTProvider := provider.NewDRRAADTProvider(provider.DRRAADTConfig{
			CSVURL: cfg.DRRAADTCSVURL, RoadLayerURL: cfg.DRRAADTRoadLayerURL, DataYear: cfg.DRRAADTYear,
			CacheTTL: cfg.DRRAADTCacheTTL, UserAgent: cfg.ExternalUserAgent,
		}, providerClient("drr_aadt"), providerCache("drr_aadt"))
		dltEVProvider := provider.NewDLTEVRegistrationProvider(provider.DLTEVRegistrationConfig{
			Geocoder: analysisGeocoder,
			CSVURL:   cfg.DLTEVRegistrationCSVURL, DatasetDate: cfg.DLTEVRegistrationDatasetDate,
			PreviousCSVURL: cfg.DLTEVRegistrationPreviousCSVURL, PreviousDatasetDate: cfg.DLTEVRegistrationPreviousDatasetDate,
			CacheTTL: cfg.DLTEVRegistrationCacheTTL, UserAgent: cfg.ExternalUserAgent,
		}, providerClientWithTimeout("dlt_ev", 90*time.Second), providerCache("dlt_ev"))
		provincialChargerProvider := provider.NewProvincialChargerProvider(provider.ProvincialChargerConfig{
			SuphanburiCSVURL: cfg.ProvincialChargerSuphanburiCSVURL,
			SaraburiJSONURL:  cfg.ProvincialChargerSaraburiJSONURL,
			CacheTTL:         cfg.ProvincialChargerCacheTTL, UserAgent: cfg.ExternalUserAgent,
		}, providerClient("provincial_charger"), providerCache("provincial_charger"))
		meaPowerMapProvider := provider.NewMEAPowerMapProvider(provider.MEAPowerMapConfig{
			PageURL: cfg.MEAPowerMapPageURL, DataURL: cfg.MEAPowerMapDataURL,
			Year: cfg.MEAPowerMapYear, VoltageKV: cfg.MEAPowerMapVoltageKV,
			CacheTTL: cfg.MEAPowerMapCacheTTL, UserAgent: cfg.ExternalUserAgent,
		}, providerClientWithTimeout("mea_power_map", 45*time.Second), providerCache("mea_power_map"))
		peaGridProvider := provider.NewPEAGridProvider(provider.PEAGridConfig{
			StationURL: cfg.PEAGridStationURL, ConductorURL: cfg.PEAGridConductorURL,
			SearchRadiusMeters: cfg.PEAGridSearchRadiusMeters, CacheTTL: cfg.PEAGridCacheTTL,
			UserAgent: cfg.ExternalUserAgent,
		}, providerClient("pea_grid"), providerCache("pea_grid"))
		googlePlacesProvider := provider.NewGooglePlacesProvider(provider.GooglePlacesConfig{
			APIKey: cfg.GoogleMapsServerAPIKey, Endpoint: cfg.GooglePlacesURL,
			CacheTTL: cfg.GooglePlacesCacheTTL, UserAgent: cfg.ExternalUserAgent, UsageRecorder: recordUsage,
		}, providerClient("google_places"), providerCache("google_places"))
		openChargeMapProvider := provider.NewOpenChargeMapProvider(provider.OpenChargeMapConfig{
			APIKey: cfg.OpenChargeMapAPIKey, Endpoint: cfg.OpenChargeMapURL,
			CacheTTL: cfg.OpenChargeMapCacheTTL, UserAgent: cfg.ExternalUserAgent,
		}, providerClient("open_charge_map"), providerCache("open_charge_map"))
		localTrafficProvider := &provider.LocalTrafficProvider{URL: cfg.LocalTrafficCSVURL, Client: providerClient("local_traffic"), Cache: providerCache("local_traffic")}
		analysisProviders := []provider.AnalysisProvider{
			provider.Instrument("osm", osmProvider, telemetryRecorder),
			provider.Instrument("google_places", googlePlacesProvider, telemetryRecorder),
			provider.Instrument("open_charge_map", openChargeMapProvider, telemetryRecorder),
			provider.Instrument("provincial_charger", provincialChargerProvider, telemetryRecorder),
			provider.Instrument("worldpop", worldPopProvider, telemetryRecorder),
			provider.Instrument("dpm_flood_history", dpmFloodHistoryProvider, telemetryRecorder),
			provider.Instrument("gistda_gateway_flood", gistdaGatewayFloodProvider, telemetryRecorder),
			provider.Instrument("gistda_elevation", gistdaElevationProvider, telemetryRecorder),
			provider.Instrument("doh_aadt", dohAADTProvider, telemetryRecorder),
			provider.Instrument("drr_aadt", drrAADTProvider, telemetryRecorder),
			provider.Instrument("local_traffic", localTrafficProvider, telemetryRecorder),
			provider.Instrument("dlt_ev", dltEVProvider, telemetryRecorder),
			provider.Instrument("mea_power_map", meaPowerMapProvider, telemetryRecorder),
			provider.Instrument("pea_grid", peaGridProvider, telemetryRecorder),
		}
		if cfg.GISTDAFloodReferenceEnabled {
			floodReference := provider.NewGISTDAFloodProvider(provider.GISTDAFloodConfig{Endpoint: cfg.GISTDAFloodRiskURL, UserAgent: cfg.ExternalUserAgent}, providerClientWithTimeout("gistda_flood_reference", 8*time.Second), providerCache("gistda_flood_reference"))
			analysisProviders = append(analysisProviders, provider.Instrument("gistda_flood_reference", floodReference, telemetryRecorder))
		}
		if cfg.GISTDADisasterEnabled {
			analysisProviders = append(analysisProviders, provider.Instrument("gistda_recent_flood", gistdaRecentFloodProvider, telemetryRecorder))
		}
		dataProvider = provider.NewCompositeProvider(analysisProviders...)
	}

	scoringEngine, err := scoring.New(scoring.DefaultWeights)
	if err != nil {
		logger.Error("invalid scoring configuration", "error", err)
		os.Exit(1)
	}
	siteService := site.NewService(repo)
	geminiAdvisory := advisory.NewGeminiService(advisory.GeminiConfig{APIKey: cfg.GeminiAPIKey, Model: cfg.GeminiModel, BaseURL: cfg.GeminiBaseURL, Timeout: cfg.GeminiTimeout}, telemetry.NewHTTPClient("gemini", cfg.GeminiTimeout, telemetryRecorder))
	analysisService := analysis.NewService(repo, dataProvider, scoringEngine, geminiAdvisory)
	analysisService.StartWorker(ctx, logger)
	authService := auth.New(repo, cfg.JWTSecret, cfg.JWTTokenTTL)
	otpService := auth.NewOTPService(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom, cfg.OTPTTL)
	lineNotifier := lineapi.NewNotifier(cfg.LINEChannelAccessToken, cfg.LINEChannelSecret, cfg.LINENotificationRecipientID, cfg.LINEAPIBaseURL, repo, nil)
	handler := httpapi.NewHandler(siteService, analysisService, repo, geocoder, geminiAdvisory, authService, otpService, lineNotifier, scoring.DefaultWeights)
	router := httpapi.NewRouter(cfg, handler)
	logger.Info("api listening", "port", cfg.Port, "environment", cfg.Environment)
	server := &http.Server{Addr: ":" + cfg.Port, Handler: router, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, WriteTimeout: 150 * time.Second, ReadTimeout: 120 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("api shutdown failed", "error", err)
		}
	}()
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}
