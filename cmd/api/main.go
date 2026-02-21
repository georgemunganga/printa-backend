package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

"github.com/georgemunganga/printa-backend/internal/modules/auth"
	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/georgemunganga/printa-backend/internal/modules/user"
	"github.com/go-chi/chi/v5"
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

	userRepo := user.NewPostgresRepository(db)
	userService := user.NewService(userRepo)
	userHandler := user.NewHandler(userService)

	authService := auth.NewService(userRepo)
	authHandler := auth.NewHandler(authService)

	router := chi.NewRouter()

	userHandler.RegisterRoutes(router)
	authHandler.RegisterRoutes(router)

	vendorTierRepo := vendor.NewTierPostgresRepository(db)
	vendorRepo := vendor.NewPostgresRepository(db)
	vendorService := vendor.NewService(vendorRepo, vendorTierRepo)
	vendorHandler := vendor.NewHandler(vendorService)
	vendorHandler.RegisterRoutes(router)

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
