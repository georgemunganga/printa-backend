package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/georgemunganga/printa-backend/internal/modules/comms"
	_ "github.com/lib/pq"
)

type reminderCandidate struct {
	VendorID       string
	OwnerID        string
	Email          string
	PeriodEnd      time.Time
	GraceEndsAt    *time.Time
	SubscriptionID string
}

func main() {
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

	communications := comms.NewService(
		comms.NewPostgresRepository(db),
		comms.NewEmailAdapter(),
		comms.NewSMSAdapter(),
		comms.NewPushAdapter(),
		comms.NewWhatsAppAdapter(),
	)
	if err := dispatch(context.Background(), db, communications, time.Now().UTC()); err != nil {
		log.Fatal("dispatch vendor subscription reminders:", err)
	}
}

func dispatch(ctx context.Context, db *sql.DB, communications comms.Service, now time.Time) error {
	candidates, err := dueCandidates(ctx, db, now)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		reminderType, subject, body, shouldSend := chooseReminder(candidate, now)
		if !shouldSend {
			continue
		}
		reminderID, shouldDeliver, err := reserveReminder(ctx, db, candidate, reminderType, now)
		if err != nil {
			return err
		}
		if !shouldDeliver {
			continue
		}
		result, err := communications.Send(ctx, comms.SendRequest{
			Channel:        comms.ChannelEmail,
			Recipient:      candidate.Email,
			RecipientID:    candidate.OwnerID,
			Subject:        subject,
			Body:           body,
			IdempotencyKey: fmt.Sprintf("vendor-reminder:%s:%s:%s:%s", candidate.VendorID, reminderType, now.Format("2006-01-02"), reminderID),
		})
		if err != nil {
			log.Printf("vendor %s reminder %s send error: %v", candidate.VendorID, reminderType, err)
			continue
		}
		if result.Status != comms.DeliverySent {
			log.Printf("vendor %s reminder %s delivery failed: %s", candidate.VendorID, reminderType, result.Error)
			continue
		}
		if _, err := db.ExecContext(ctx, `UPDATE vendor_operating_reminders SET sent_at = $1 WHERE id = $2 AND sent_at IS NULL`, now, reminderID); err != nil {
			return fmt.Errorf("mark reminder sent: %w", err)
		}
		log.Printf("sent %s reminder to vendor %s", reminderType, candidate.VendorID)
	}
	return nil
}

func dueCandidates(ctx context.Context, db *sql.DB, now time.Time) ([]reminderCandidate, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT v.id, u.id, u.email, s.current_period_end, s.id, g.ends_at
		FROM vendors v
		JOIN users u ON u.id = v.owner_id
		JOIN vendor_subscriptions s ON s.vendor_id = v.id
		LEFT JOIN vendor_subscription_grace_periods g
		  ON g.vendor_id = v.id AND g.status = 'ACTIVE' AND g.ends_at > $1
		WHERE s.status = 'PAST_DUE'
		  AND s.current_period_end < $1
		  AND NULLIF(trim(COALESCE(u.email, '')), '') IS NOT NULL`, now)
	if err != nil {
		return nil, fmt.Errorf("query overdue subscription candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]reminderCandidate, 0)
	for rows.Next() {
		var candidate reminderCandidate
		var graceEndsAt sql.NullTime
		if err := rows.Scan(&candidate.VendorID, &candidate.OwnerID, &candidate.Email, &candidate.PeriodEnd, &candidate.SubscriptionID, &graceEndsAt); err != nil {
			return nil, fmt.Errorf("scan overdue subscription candidate: %w", err)
		}
		if graceEndsAt.Valid {
			candidate.GraceEndsAt = &graceEndsAt.Time
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate overdue subscription candidates: %w", err)
	}
	return candidates, nil
}

func chooseReminder(candidate reminderCandidate, now time.Time) (reminderType, subject, body string, shouldSend bool) {
	if candidate.GraceEndsAt != nil {
		if candidate.GraceEndsAt.Sub(now) <= 24*time.Hour {
			return "GRACE_EXPIRING", "Your Printa subscription grace period ends soon", fmt.Sprintf("Your five-day Printa subscription grace period ends on %s. Please arrange subscription payment to avoid an interruption to vendor operations. Subscription payment collection is not yet available in Printa; contact support from your vendor portal for payment assistance.", candidate.GraceEndsAt.Format(time.RFC1123)), true
		}
		return "", "", "", false
	}

	daysPastDue := int(now.Sub(candidate.PeriodEnd).Hours() / 24)
	if daysPastDue < 0 || (daysPastDue >= 3 && daysPastDue%2 != 0) {
		return "", "", "", false
	}
	return "SUBSCRIPTION_DUE", "Your Printa subscription payment is due", "Your Printa subscription is overdue and vendor operations remain paused until payment is recorded. If you are eligible, sign in to request your one automatic five-day grace period. Subscription payment collection is not yet available in Printa; contact support from your vendor portal for payment assistance.", true
}

func reserveReminder(ctx context.Context, db *sql.DB, candidate reminderCandidate, reminderType string, now time.Time) (string, bool, error) {
	var reminderID string
	var sentAt sql.NullTime
	err := db.QueryRowContext(ctx, `
		INSERT INTO vendor_operating_reminders (vendor_id, reminder_type, delivery_day, recipient)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (vendor_id, reminder_type, delivery_day) DO NOTHING
		RETURNING id`, candidate.VendorID, reminderType, now.Format("2006-01-02"), candidate.Email).Scan(&reminderID)
	if err == nil {
		return reminderID, true, nil
	}
	if err != sql.ErrNoRows {
		return "", false, fmt.Errorf("reserve reminder: %w", err)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT id, sent_at FROM vendor_operating_reminders
		WHERE vendor_id = $1 AND reminder_type = $2 AND delivery_day = $3`, candidate.VendorID, reminderType, now.Format("2006-01-02")).Scan(&reminderID, &sentAt); err != nil {
		return "", false, fmt.Errorf("read reminder reservation: %w", err)
	}
	return reminderID, !sentAt.Valid, nil
}
