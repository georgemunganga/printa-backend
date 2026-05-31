package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	appMiddleware "github.com/georgemunganga/printa-backend/internal/middleware"
	"github.com/georgemunganga/printa-backend/internal/modules/auth"
	"github.com/georgemunganga/printa-backend/internal/modules/billing"
	"github.com/georgemunganga/printa-backend/internal/modules/catalog"
	"github.com/georgemunganga/printa-backend/internal/modules/inventory"
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

	// ── Services & Repositories ──────────────────────────────
	userRepo := user.NewPostgresRepository(db)
	userService := user.NewService(userRepo)
	authService := auth.NewService(userRepo)

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

	// ── PUBLIC ROUTES (no auth required) ────────────────────
	// User registration
	user.NewHandler(userService).RegisterPublicRoutes(router)
	// Login
	auth.NewHandler(authService).RegisterRoutes(router)
	// Payment webhooks (provider-signed, no JWT)
	payment.NewHandler(paymentService).RegisterWebhookRoutes(router)

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
		inventory.NewHandler(inventoryService).RegisterRoutes(r)

		// Orders
		order.NewHandler(orderService).RegisterRoutes(r)

		// Routing engine
		routing.NewHandler(routingService).RegisterRoutes(r)

		// Production
		production.NewHandler(productionService).RegisterRoutes(r)

		// POS
		pos.NewHandler(posService).RegisterRoutes(r)

		// Billing
		billing.NewHandler(billingService).RegisterRoutes(r)

		// Payments (protected)
		payment.NewHandler(paymentService).RegisterProtectedRoutes(r)
	})

	// ── Start Server ─────────────────────────────────────────
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Printa API server starting on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
