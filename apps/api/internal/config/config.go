package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment                          string
	Port                                 string
	DatabaseURL                          string
	RedisURL                             string
	RedisCacheTTL                        time.Duration
	CORSAllowedOrigins                   []string
	AnalysisProviderMode                 string
	OverpassURL                          string
	OverpassFallbackURLs                 []string
	NominatimURL                         string
	NominatimCountryCodes                string
	GeocodingCacheTTL                    time.Duration
	ExternalUserAgent                    string
	ExternalHTTPTimeout                  time.Duration
	WorldPopURL                          string
	WorldPopYear                         int
	WorldPopResolution                   string
	WorldPopCacheTTL                     time.Duration
	GISTDAAPIKey                         string
	GISTDAElevationURL                   string
	GISTDAElevationCacheTTL              time.Duration
	GISTDAFloodRiskURL                   string
	GISTDAFloodCacheTTL                  time.Duration
	DOHAADTCSVURL                        string
	DOHAADTRoadLayerURL                  string
	DOHAADTYear                          int
	DOHAADTCacheTTL                      time.Duration
	DRRAADTCSVURL                        string
	LocalTrafficCSVURL                   string
	DRRAADTRoadLayerURL                  string
	DRRAADTYear                          int
	DRRAADTCacheTTL                      time.Duration
	DLTEVRegistrationCSVURL              string
	DLTEVRegistrationDatasetDate         string
	DLTEVRegistrationPreviousCSVURL      string
	DLTEVRegistrationPreviousDatasetDate string
	DLTEVRegistrationCacheTTL            time.Duration
	ProvincialChargerSuphanburiCSVURL    string
	ProvincialChargerSaraburiJSONURL     string
	ProvincialChargerCacheTTL            time.Duration
	MEAPowerMapPageURL                   string
	MEAPowerMapDataURL                   string
	MEAPowerMapYear                      int
	MEAPowerMapVoltageKV                 int
	MEAPowerMapCacheTTL                  time.Duration
	PEAPowerMapURL                       string
	PEAGridStationURL                    string
	PEAGridConductorURL                  string
	PEAGridSearchRadiusMeters            int
	PEAGridCacheTTL                      time.Duration
	GoogleMapsServerAPIKey               string
	GooglePlacesURL                      string
	GooglePlacesCacheTTL                 time.Duration
	OpenChargeMapAPIKey                  string
	OpenChargeMapURL                     string
	OpenChargeMapCacheTTL                time.Duration
	GeminiAPIKey                         string
	GeminiModel                          string
	GeminiBaseURL                        string
	GeminiTimeout                        time.Duration
	JWTSecret                            string
	AuthRequired                         bool
	SMTPHost                             string
	SMTPPort                             string
	SMTPUsername                         string
	SMTPPassword                         string
	SMTPFrom                             string
	OTPTTL                               time.Duration
	LINEChannelAccessToken               string
	LINEChannelSecret                    string
	LINENotificationRecipientID          string
	LINEAPIBaseURL                       string
}

func Load() Config {
	ttl, err := time.ParseDuration(getEnv("REDIS_CACHE_TTL", "15m"))
	if err != nil {
		ttl = 15 * time.Minute
	}

	origins := strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"), ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	// A 3 km OpenStreetMap/Overpass evidence query can legitimately take more
	// than 20 seconds in dense areas. Keep the limit bounded, but long enough
	// that a healthy public endpoint is not incorrectly labelled unavailable.
	httpTimeout, err := time.ParseDuration(getEnv("EXTERNAL_HTTP_TIMEOUT", "45s"))
	if err != nil {
		httpTimeout = 45 * time.Second
	}
	geocodingTTL, err := time.ParseDuration(getEnv("GEOCODING_CACHE_TTL", "720h"))
	if err != nil {
		geocodingTTL = 30 * 24 * time.Hour
	}
	worldPopTTL, err := time.ParseDuration(getEnv("WORLDPOP_CACHE_TTL", "720h"))
	if err != nil {
		worldPopTTL = 30 * 24 * time.Hour
	}
	gistdaFloodTTL, err := time.ParseDuration(getEnv("GISTDA_FLOOD_CACHE_TTL", "24h"))
	if err != nil {
		gistdaFloodTTL = 24 * time.Hour
	}
	gistdaElevationTTL, err := time.ParseDuration(getEnv("GISTDA_ELEVATION_CACHE_TTL", "24h"))
	if err != nil {
		gistdaElevationTTL = 24 * time.Hour
	}
	dohAADTTTL, err := time.ParseDuration(getEnv("DOH_AADT_CACHE_TTL", "24h"))
	if err != nil {
		dohAADTTTL = 24 * time.Hour
	}
	dohAADTYear := 2568
	if parsed, parseErr := strconv.Atoi(getEnv("DOH_AADT_YEAR", "2568")); parseErr == nil {
		dohAADTYear = parsed
	}
	drrAADTTTL, err := time.ParseDuration(getEnv("DRR_AADT_CACHE_TTL", "24h"))
	if err != nil {
		drrAADTTTL = 24 * time.Hour
	}
	drrAADTYear := 2568
	if parsed, parseErr := strconv.Atoi(getEnv("DRR_AADT_YEAR", "2568")); parseErr == nil {
		drrAADTYear = parsed
	}
	dltEVTTL, err := time.ParseDuration(getEnv("DLT_EV_REGISTRATION_CACHE_TTL", "720h"))
	if err != nil {
		dltEVTTL = 30 * 24 * time.Hour
	}
	provincialChargerTTL, err := time.ParseDuration(getEnv("PROVINCIAL_CHARGER_CACHE_TTL", "24h"))
	if err != nil {
		provincialChargerTTL = 24 * time.Hour
	}
	meaPowerMapTTL, err := time.ParseDuration(getEnv("MEA_POWER_MAP_CACHE_TTL", "24h"))
	if err != nil {
		meaPowerMapTTL = 24 * time.Hour
	}
	peaGridTTL, err := time.ParseDuration(getEnv("PEA_GRID_CACHE_TTL", "24h"))
	if err != nil {
		peaGridTTL = 24 * time.Hour
	}
	googlePlacesTTL, err := time.ParseDuration(getEnv("GOOGLE_PLACES_CACHE_TTL", "24h"))
	if err != nil {
		googlePlacesTTL = 24 * time.Hour
	}
	openChargeMapTTL, err := time.ParseDuration(getEnv("OPEN_CHARGE_MAP_CACHE_TTL", "24h"))
	if err != nil {
		openChargeMapTTL = 24 * time.Hour
	}
	peaGridSearchRadius := 25000
	if parsed, parseErr := strconv.Atoi(getEnv("PEA_GRID_SEARCH_RADIUS_METERS", "25000")); parseErr == nil && parsed > 0 {
		peaGridSearchRadius = parsed
	}
	meaPowerMapYear := time.Now().Year()
	if parsed, parseErr := strconv.Atoi(getEnv("MEA_POWER_MAP_YEAR", strconv.Itoa(meaPowerMapYear))); parseErr == nil {
		meaPowerMapYear = parsed
	}
	meaPowerMapVoltageKV := 115
	if parsed, parseErr := strconv.Atoi(getEnv("MEA_POWER_MAP_VOLTAGE_KV", "115")); parseErr == nil {
		meaPowerMapVoltageKV = parsed
	}
	geminiTimeout, err := time.ParseDuration(getEnv("GEMINI_TIMEOUT", "20s"))
	if err != nil {
		geminiTimeout = 20 * time.Second
	}
	otpTTL, err := time.ParseDuration(getEnv("OTP_TTL", "10m"))
	if err != nil {
		otpTTL = 10 * time.Minute
	}
	worldPopYear := time.Now().Year()
	if parsed, parseErr := strconv.Atoi(getEnv("WORLDPOP_YEAR", strconv.Itoa(worldPopYear))); parseErr == nil {
		worldPopYear = parsed
	}

	return Config{
		Environment:                          getEnv("APP_ENV", "development"),
		Port:                                 getEnv("API_PORT", "8080"),
		DatabaseURL:                          os.Getenv("DATABASE_URL"),
		RedisURL:                             os.Getenv("REDIS_URL"),
		RedisCacheTTL:                        ttl,
		CORSAllowedOrigins:                   origins,
		AnalysisProviderMode:                 getEnv("ANALYSIS_PROVIDER_MODE", "unavailable"),
		OverpassURL:                          getEnv("OVERPASS_API_URL", "https://overpass-api.de/api/interpreter"),
		OverpassFallbackURLs:                 overpassFallbackURLs(),
		NominatimURL:                         getEnv("NOMINATIM_API_URL", "https://nominatim.openstreetmap.org/search"),
		NominatimCountryCodes:                getEnv("NOMINATIM_COUNTRY_CODES", "th"),
		GeocodingCacheTTL:                    geocodingTTL,
		ExternalUserAgent:                    getEnv("EXTERNAL_USER_AGENT", "RBC-EV-Station-MVP/0.1 (data-team@rbc.local)"),
		ExternalHTTPTimeout:                  httpTimeout,
		WorldPopURL:                          getEnv("WORLDPOP_API_URL", "https://api.worldpop.org/v2"),
		WorldPopYear:                         worldPopYear,
		WorldPopResolution:                   getEnv("WORLDPOP_RESOLUTION", "100m"),
		WorldPopCacheTTL:                     worldPopTTL,
		GISTDAAPIKey:                         os.Getenv("GISTDA_API_KEY"),
		GISTDAElevationURL:                   getEnv("GISTDA_ELEVATION_URL", "https://api.sphere.gistda.or.th/services/geo/elevation"),
		GISTDAElevationCacheTTL:              gistdaElevationTTL,
		GISTDAFloodRiskURL:                   getEnv("GISTDA_FLOOD_RISK_URL", "https://gistdaportal.gistda.or.th/arcgis/rest/services/app/GISTDA_flood/MapServer/1/query"),
		GISTDAFloodCacheTTL:                  gistdaFloodTTL,
		DOHAADTCSVURL:                        getEnv("DOH_AADT_CSV_URL", "https://opendata.doh.go.th/dataset/ed101df4-f0a1-4d7f-b9d9-76859b2ca73e/resource/f88f9c4e-b32e-4cd1-bf51-4e1f84ba7b16/download/aadt-68.csv"),
		DOHAADTRoadLayerURL:                  getEnv("DOH_AADT_ROAD_LAYER_URL", "https://giportal.mot.go.th/arcgis/rest/services/Hosted/%E0%B9%80%E0%B8%AA%E0%B9%89%E0%B8%99%E0%B8%97%E0%B8%B2%E0%B8%87%E0%B8%AB%E0%B8%A5%E0%B8%A7%E0%B8%87%E0%B9%81%E0%B8%9C%E0%B9%88%E0%B8%99%E0%B8%94%E0%B8%B4%E0%B8%99_%E0%B8%97%E0%B8%A5/FeatureServer/0/query"),
		DOHAADTYear:                          dohAADTYear,
		DOHAADTCacheTTL:                      dohAADTTTL,
		DRRAADTCSVURL:                        getEnv("DRR_AADT_CSV_URL", "https://dataportal.drr.go.th/dataset/3833590c-a0f4-4c99-baf1-8c540aadebb5/resource/0d612f16-e188-4604-b61c-c0da07854ef9/download/untitled.csv"),
		LocalTrafficCSVURL:                   getEnv("LOCAL_TRAFFIC_CSV_URL", ""),
		DRRAADTRoadLayerURL:                  getEnv("DRR_AADT_ROAD_LAYER_URL", "https://gis.drr.go.th/arcgis/rest/services/DRR_Feature/FeatureServer/2/query"),
		DRRAADTYear:                          drrAADTYear,
		DRRAADTCacheTTL:                      drrAADTTTL,
		DLTEVRegistrationCSVURL:              getEnv("DLT_EV_REGISTRATION_CSV_URL", "https://gdcatalog.dlt.go.th/dataset/e8b7d87e-5a3c-4b8b-854c-974d3b600256/resource/e5a037c6-3cc2-415e-b559-e6e85cfba2ca/download/stt_car_fuel_at_25690228.csv"),
		DLTEVRegistrationDatasetDate:         getEnv("DLT_EV_REGISTRATION_DATASET_DATE", "28 February 2569"),
		DLTEVRegistrationPreviousCSVURL:      strings.TrimSpace(os.Getenv("DLT_EV_REGISTRATION_PREVIOUS_CSV_URL")),
		DLTEVRegistrationPreviousDatasetDate: strings.TrimSpace(os.Getenv("DLT_EV_REGISTRATION_PREVIOUS_DATASET_DATE")),
		DLTEVRegistrationCacheTTL:            dltEVTTL,
		ProvincialChargerSuphanburiCSVURL:    getEnv("PROVINCIAL_CHARGER_SUPHANBURI_CSV_URL", "https://suphanburi.gdcatalog.go.th/dataset/c90ca9d9-b1ed-4f0d-bf08-728bac7b72fd/resource/9f1855c3-94d2-478e-95f9-63cc896330bb/download/untitled.csv"),
		ProvincialChargerSaraburiJSONURL:     getEnv("PROVINCIAL_CHARGER_SARABURI_JSON_URL", "https://energy.saraburidev.org/api/public/stations"),
		ProvincialChargerCacheTTL:            provincialChargerTTL,
		MEAPowerMapPageURL:                   getEnv("MEA_POWER_MAP_PAGE_URL", "https://measervice.mea.or.th/powermap/load/index.html"),
		MEAPowerMapDataURL:                   getEnv("MEA_POWER_MAP_DATA_URL", "https://measervice.mea.or.th/powermap/load/powermap_data.js"),
		MEAPowerMapYear:                      meaPowerMapYear,
		MEAPowerMapVoltageKV:                 meaPowerMapVoltageKV,
		MEAPowerMapCacheTTL:                  meaPowerMapTTL,
		PEAPowerMapURL:                       getEnv("PEA_POWER_MAP_URL", "https://ppmload.pea.co.th/"),
		PEAGridStationURL:                    getEnv("PEA_GRID_STATION_URL", "https://gisportal.pea.co.th/arcgis/rest/services/PEA_PAPD/PAPD_T_Station/FeatureServer/0/query"),
		PEAGridConductorURL:                  getEnv("PEA_GRID_CONDUCTOR_URL", "https://gisportal.pea.co.th/arcgis/rest/services/PEA_PAPD/PAPD_T_Station/FeatureServer/1/query"),
		PEAGridSearchRadiusMeters:            peaGridSearchRadius,
		PEAGridCacheTTL:                      peaGridTTL,
		GoogleMapsServerAPIKey:               strings.TrimSpace(os.Getenv("GOOGLE_MAPS_SERVER_API_KEY")),
		GooglePlacesURL:                      getEnv("GOOGLE_PLACES_URL", "https://places.googleapis.com/v1/places:searchNearby"),
		GooglePlacesCacheTTL:                 googlePlacesTTL,
		OpenChargeMapAPIKey:                  strings.TrimSpace(os.Getenv("OPEN_CHARGE_MAP_API_KEY")),
		OpenChargeMapURL:                     getEnv("OPEN_CHARGE_MAP_API_URL", "https://api.openchargemap.io/v3/poi/"),
		OpenChargeMapCacheTTL:                openChargeMapTTL,
		GeminiAPIKey:                         strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		GeminiModel:                          getEnv("GEMINI_MODEL", "gemini-3.5-flash-lite"),
		GeminiBaseURL:                        getEnv("GEMINI_BASE_URL", "https://generativelanguage.googleapis.com/v1beta"),
		GeminiTimeout:                        geminiTimeout,
		JWTSecret:                            getEnv("JWT_SECRET", "development-only-change-this-secret"),
		AuthRequired:                         strings.EqualFold(getEnv("AUTH_REQUIRED", "false"), "true"),
		SMTPHost:                             os.Getenv("SMTP_HOST"), SMTPPort: getEnv("SMTP_PORT", "587"), SMTPUsername: os.Getenv("SMTP_USERNAME"), SMTPPassword: os.Getenv("SMTP_PASSWORD"), SMTPFrom: os.Getenv("SMTP_FROM"), OTPTTL: otpTTL,
		LINEChannelAccessToken: strings.TrimSpace(os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")), LINEChannelSecret: strings.TrimSpace(os.Getenv("LINE_CHANNEL_SECRET")), LINENotificationRecipientID: strings.TrimSpace(os.Getenv("LINE_NOTIFICATION_RECIPIENT_ID")), LINEAPIBaseURL: getEnv("LINE_API_BASE_URL", "https://api.line.me"),
	}
}

func splitURLs(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func overpassFallbackURLs() []string {
	configured := splitURLs(os.Getenv("OVERPASS_FALLBACK_API_URLS"))
	if len(configured) > 0 {
		return configured
	}
	return []string{
		"https://overpass.kumi.systems/api/interpreter",
		"https://overpass.private.coffee/api/interpreter",
	}
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
