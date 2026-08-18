package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/georgemunganga/printa-backend/internal/modules/comms"
	"github.com/georgemunganga/printa-backend/internal/modules/notification"
	"github.com/georgemunganga/printa-backend/internal/outbox"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	_ = godotenv.Load()
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatal("open database:", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal("database connection failed:", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	commsService := comms.NewService(
		comms.NewPostgresRepository(db),
		comms.NewEmailAdapter(),
		comms.NewSMSAdapter(),
		comms.NewPushAdapter(),
		comms.NewWhatsAppAdapter(),
	)
	worker := &outbox.Worker{
		Repository: outbox.NewRepository(db),
		Handlers: map[string]outbox.Handler{
			"notification.dispatch.v1": notificationDispatchHandler(commsService),
		},
		PollEvery:   durationEnv("OUTBOX_POLL_INTERVAL", 2*time.Second),
		LeaseFor:    durationEnv("OUTBOX_LEASE_DURATION", 5*time.Minute),
		BatchSize:   intEnv("OUTBOX_BATCH_SIZE", 25),
		MaxAttempts: intEnv("OUTBOX_MAX_ATTEMPTS", 5),
		Logger:      log.Default(),
	}
	log.Printf("Printa outbox worker starting (poll=%s lease=%s batch=%d max_attempts=%d)", worker.PollEvery, worker.LeaseFor, worker.BatchSize, worker.MaxAttempts)
	if err := worker.Run(ctx); err != nil {
		log.Fatal("outbox worker stopped with error:", err)
	}
	log.Println("Printa outbox worker stopped")
}

func notificationDispatchHandler(commsService comms.Service) outbox.Handler {
	return func(ctx context.Context, event outbox.Event) error {
		var notificationEvent notification.Event
		if err := json.Unmarshal(event.Payload, &notificationEvent); err != nil {
			return fmt.Errorf("decode notification event: %w", err)
		}
		if notificationEvent.RecipientID == "" || notificationEvent.Type == "" {
			return errors.New("notification event requires recipient_id and type")
		}
		return commsService.SendEvent(ctx, notificationEvent)
	}
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		log.Printf("invalid %s=%q; using %s", key, value, fallback)
		return fallback
	}
	return parsed
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		log.Printf("invalid %s=%q; using %d", key, value, fallback)
		return fallback
	}
	return parsed
}
