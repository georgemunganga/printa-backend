package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/georgemunganga/printa-backend/internal/apidocs"
	appMiddleware "github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/admin"
	"github.com/georgemunganga/printa-backend/internal/modules/assets"
	"github.com/georgemunganga/printa-backend/internal/modules/auth"
	"github.com/georgemunganga/printa-backend/internal/modules/billing"
	"github.com/georgemunganga/printa-backend/internal/modules/catalog"
	"github.com/georgemunganga/printa-backend/internal/modules/comms"
	"github.com/georgemunganga/printa-backend/internal/modules/inventory"
	"github.com/georgemunganga/printa-backend/internal/modules/notification"
	"github.com/georgemunganga/printa-backend/internal/modules/order"
	"github.com/georgemunganga/printa-backend/internal/modules/payment"
	"github.com/georgemunganga/printa-backend/internal/modules/pos"
	"github.com/georgemunganga/printa-backend/internal/modules/production"
	"github.com/georgemunganga/printa-backend/internal/modules/routing"
	"github.com/georgemunganga/printa-backend/internal/modules/user"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}
	if err := validateStartupConfig(); err != nil {
		log.Fatal("Invalid configuration:", err)
	}

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Database connection failed:", err)
	}
	fmt.Println("Successfully connected to the database!")

	// ── Router ──────────────────────────────────────────────
	router := chi.NewRouter()
	router.Use(chiMiddleware.Logger)
	router.Use(chiMiddleware.Recoverer)
	router.Use(chiMiddleware.RequestID)
	router.Use(requestCorrelationMiddleware)
	router.Use(corsMiddleware())

	// ── Services & Repositories ──────────────────────────────
	userRepo := user.NewPostgresRepository(db)
	userService := user.NewService(userRepo)

	vendorTierRepo := vendor.NewTierPostgresRepository(db)
	vendorRepo := vendor.NewPostgresRepository(db)
	vendorService := vendor.NewService(vendorRepo, vendorTierRepo)

	catalogRepo := catalog.NewPostgresRepository(db)
	catalogService := catalog.NewService(catalogRepo)

	storeRepo := inventory.NewStorePostgresRepository(db)
	staffRepo := inventory.NewStoreStaffPostgresRepository(db)
	productRepo := inventory.NewProductPostgresRepository(db)
	inventoryService := inventory.NewService(storeRepo, staffRepo, productRepo)

	orderRepo := order.NewPostgresRepository(db)
	orderService := order.NewService(orderRepo)

	routingRepo := routing.NewPostgresRepository(db)
	routingService := routing.NewService(routingRepo)

	productionRepo := production.NewPostgresRepository(db)
	productionService := production.NewService(productionRepo)

	posRepo := pos.NewPostgresRepository(db)
	posService := pos.NewService(posRepo)

	billingRepo := billing.NewPostgresRepository(db)
	billingService := billing.NewService(billingRepo)

	adminRepo := admin.NewPostgresRepository(db)
	adminService := admin.NewService(adminRepo)

	notificationRepo := notification.NewPostgresRepository(db)
	commsRepo := comms.NewPostgresRepository(db)
	commsService := comms.NewService(commsRepo,
		comms.NewEmailAdapter(),
		comms.NewSMSAdapter(),
		comms.NewPushAdapter(),
		comms.NewWhatsAppAdapter(),
	)
	notificationService := notification.NewService(notificationRepo, comms.NewDispatcher(commsService))
	authService := auth.NewService(
		userRepo,
		userService,
		auth.NewPostgresOTPRepository(db),
		auth.NewPostgresOAuthRepository(db),
		commsService,
	)

	paymentGateways := payment.GatewayRegistry{
		payment.ProviderMTNMomo: payment.NewMTNMomoGateway(
			os.Getenv("MTN_MOMO_API_KEY"),
			os.Getenv("MTN_MOMO_API_SECRET"),
			os.Getenv("MTN_MOMO_BASE_URL"),
			os.Getenv("MTN_MOMO_ENV"),
		),
		payment.ProviderAirtel: payment.NewAirtelMoneyGateway(
			os.Getenv("AIRTEL_CLIENT_ID"),
			os.Getenv("AIRTEL_CLIENT_SECRET"),
			os.Getenv("AIRTEL_BASE_URL"),
			os.Getenv("AIRTEL_ENV"),
		),
	}
	paymentRepo := payment.NewPostgresRepository(db)
	paymentService := payment.NewService(paymentRepo, paymentGateways)

	assetHandler, err := assets.NewHandler(db)
	if err != nil {
		log.Fatal("Asset storage configuration failed:", err)
	}

	// ── PUBLIC ROUTES (no auth required) ────────────────────
	router.Get("/", statusPage(db))
	router.Get("/livez", livenessCheck())
	router.Get("/readyz", readinessCheck(db))
	router.Get("/healthz", readinessCheck(db))
	router.Get("/api/v1/openapi.yaml", apidocs.OpenAPIHandler)
	router.Get("/api/v1/docs", apidocs.DocsHandler)
	// User registration
	user.NewHandler(userService).RegisterPublicRoutes(router)
	// Login
	auth.NewHandler(authService).RegisterRoutes(router)
	// Customer storefront browsing
	inventory.NewHandler(inventoryService, vendorService).RegisterStorefrontRoutes(router)
	// Payment webhooks (provider callback boundary, no JWT)
	payment.NewHandler(paymentService, vendorService).RegisterWebhookRoutes(router)

	// ── PROTECTED ROUTES (JWT required) ─────────────────────
	router.Group(func(r chi.Router) {
		r.Use(appMiddleware.Authenticate)

		// Users
		user.NewHandler(userService).RegisterProtectedRoutes(r)

		// Vendor management
		vendor.NewHandler(vendorService).RegisterRoutes(r)

		// Catalog
		catalog.NewHandler(catalogService).RegisterRoutes(r)

		// Inventory
		inventory.NewHandler(inventoryService, vendorService).RegisterRoutes(r)

		// Orders
		order.NewHandler(orderService).RegisterRoutes(r)

		// Customer-owned design assets
		assetHandler.RegisterRoutes(r)

		// Routing engine
		routing.NewHandler(routingService).RegisterRoutes(r)

		// Production
		production.NewHandler(productionService, inventoryService, vendorService).RegisterRoutes(r)

		// POS
		pos.NewHandler(posService, inventoryService, vendorService).RegisterRoutes(r)

		// Billing
		billing.NewHandler(billingService, vendorService).RegisterRoutes(r)

		// Admin platform management
		admin.NewHandler(adminService).RegisterRoutes(r)
		notification.NewHandler(notificationService).RegisterRoutes(r)
		comms.NewHandler(commsService).RegisterRoutes(r)

		// Payments (protected)
		payment.NewHandler(paymentService, vendorService).RegisterProtectedRoutes(r)
	})

	// ── Start Server ─────────────────────────────────────────
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Printa API server starting on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func statusPage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "connected"
		if err := db.PingContext(r.Context()); err != nil {
			dbStatus = "unavailable"
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Printa API Status</title>
  <style>
    body { margin: 0; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #0f172a; color: #e5e7eb; }
    main { min-height: 100vh; display: grid; place-items: center; padding: 32px; box-sizing: border-box; }
    section { width: min(720px, 100%%); border: 1px solid #334155; border-radius: 8px; padding: 28px; background: #111827; }
    h1 { margin: 0 0 12px; font-size: 32px; line-height: 1.1; }
    p { margin: 8px 0; color: #cbd5e1; font-size: 16px; }
    dl { display: grid; grid-template-columns: 140px 1fr; gap: 10px 18px; margin: 24px 0 0; }
    dt { color: #94a3b8; }
    dd { margin: 0; color: #f8fafc; }
    .ok { color: #86efac; font-weight: 700; }
  </style>
</head>
<body>
  <main>
    <section>
      <h1>Printa API</h1>
      <p class="ok">Everything is live and working.</p>
      <dl>
        <dt>Service</dt><dd>online</dd>
        <dt>Database</dt><dd>%s</dd>
        <dt>Environment</dt><dd>%s</dd>
        <dt>Checked at</dt><dd>%s</dd>
      </dl>
    </section>
  </main>
</body>
</html>`, dbStatus, getenvDefault("APP_ENV", "production"), time.Now().UTC().Format(time.RFC3339))
	}
}

func livenessCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"service": "printa-api", "status": "live"})
	}
}

func readinessCheck(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusOK
		dbStatus := "connected"
		if err := db.PingContext(r.Context()); err != nil {
			status = http.StatusServiceUnavailable
			dbStatus = "unavailable"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"service":     "printa-api",
			"status":      "ready",
			"database":    dbStatus,
			"environment": getenvDefault("APP_ENV", "production"),
			"checked_at":  time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func corsMiddleware() func(http.Handler) http.Handler {
	allowedOrigins := configuredCORSOrigins()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allowedOrigins[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Requested-With")
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func configuredCORSOrigins() map[string]bool {
	defaults := []string{
		"https://vendor.printa.co.zm",
		"https://app.printa.co.zm",
		"https://printa.co.zm",
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://localhost:5174",
		"http://127.0.0.1:5174",
	}
	origins := make(map[string]bool, len(defaults))
	for _, origin := range defaults {
		origins[origin] = true
	}
	for _, origin := range splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS")) {
		origins[origin] = true
	}
	return origins
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func requestCorrelationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestID := chiMiddleware.GetReqID(r.Context()); requestID != "" {
			w.Header().Set("X-Request-ID", requestID)
		}
		next.ServeHTTP(w, r)
	})
}

func validateStartupConfig() error {
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if getenvDefault("APP_ENV", "development") == "production" && strings.TrimSpace(os.Getenv("JWT_SECRET")) == "" {
		return fmt.Errorf("JWT_SECRET is required in production")
	}
	return nil
}

func getenvDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
