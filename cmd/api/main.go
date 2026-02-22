package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/georgemunganga/printa-backend/internal/modules/auth"
	"github.com/georgemunganga/printa-backend/internal/modules/catalog"
	"github.com/georgemunganga/printa-backend/internal/modules/inventory"
	"github.com/georgemunganga/printa-backend/internal/modules/order"
	"github.com/georgemunganga/printa-backend/internal/modules/billing"
	"github.com/georgemunganga/printa-backend/internal/modules/payment"
	"github.com/georgemunganga/printa-backend/internal/modules/pos"
	"github.com/georgemunganga/printa-backend/internal/modules/production"
	"github.com/georgemunganga/printa-backend/internal/modules/routing"
	"github.com/georgemunganga/printa-backend/internal/modules/user"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Successfully connected to the database!")

	// ── Router ──────────────────────────────────────────────
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)

	// ── Phase 1: Identity & Business ────────────────────────
	userRepo := user.NewPostgresRepository(db)
	userService := user.NewService(userRepo)
	user.NewHandler(userService).RegisterRoutes(router)

	authService := auth.NewService(userRepo)
	auth.NewHandler(authService).RegisterRoutes(router)

	vendorTierRepo := vendor.NewTierPostgresRepository(db)
	vendorRepo := vendor.NewPostgresRepository(db)
	vendorService := vendor.NewService(vendorRepo, vendorTierRepo)
	vendor.NewHandler(vendorService).RegisterRoutes(router)

	// ── Phase 2: Catalog & Inventory ────────────────────────
	catalogRepo := catalog.NewPostgresRepository(db)
	catalogService := catalog.NewService(catalogRepo)
	catalog.NewHandler(catalogService).RegisterRoutes(router)

	storeRepo := inventory.NewStorePostgresRepository(db)
	staffRepo := inventory.NewStoreStaffPostgresRepository(db)
	productRepo := inventory.NewProductPostgresRepository(db)
	inventoryService := inventory.NewService(storeRepo, staffRepo, productRepo)
	inventory.NewHandler(inventoryService).RegisterRoutes(router)

	// ── Phase 3: Order Management ───────────────────────────
	orderRepo := order.NewPostgresRepository(db)
	orderService := order.NewService(orderRepo)
	order.NewHandler(orderService).RegisterRoutes(router)

	// ── Phase 4: Deterministic Routing Engine ─────────────────
	routingRepo := routing.NewPostgresRepository(db)
	routingService := routing.NewService(routingRepo)
	routing.NewHandler(routingService).RegisterRoutes(router)

	// ── Phase 5: Production & POS ───────────────────────────
	productionRepo := production.NewPostgresRepository(db)
	productionService := production.NewService(productionRepo)
	production.NewHandler(productionService).RegisterRoutes(router)

	posRepo := pos.NewPostgresRepository(db)
	posService := pos.NewService(posRepo)
	pos.NewHandler(posService).RegisterRoutes(router)

	// ── Phase 6: Vendor Subscriptions & Billing ──────────────────
	billingRepo := billing.NewPostgresRepository(db)
	billingService := billing.NewService(billingRepo)
	billing.NewHandler(billingService).RegisterRoutes(router)

	// ── Phase 7: Pluggable Payments ─────────────────────────────
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
	payment.NewHandler(paymentService).RegisterRoutes(router)

	// ── Start Server ─────────────────────────────────────────
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Printa API server starting on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
