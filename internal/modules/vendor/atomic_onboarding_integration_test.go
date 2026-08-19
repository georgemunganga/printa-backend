package vendor

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// TestOnboardVendorWithFirstStoreAtomicity exercises the real PostgreSQL
// transaction when DATABASE_URL is configured. It skips in isolated unit-test
// environments and cleans up every temporary user through the FK cascade.
func TestOnboardVendorWithFirstStoreAtomicity(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	ctx := context.Background()
	createVendorUser := func(t *testing.T) string {
		t.Helper()
		id := uuid.New()
		email := "atomic-onboarding-" + id.String() + "@example.invalid"
		if _, err := db.ExecContext(ctx, `
			INSERT INTO users (id, email, password_hash, first_name, last_name, role, is_active)
			VALUES ($1, $2, $3, $4, $5, 'VENDOR', TRUE)
		`, id, email, "not-used", "Atomic", "Test"); err != nil {
			t.Fatalf("create temporary vendor user: %v", err)
		}
		t.Cleanup(func() {
			if _, err := db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
				t.Errorf("clean up temporary user: %v", err)
			}
		})
		return id.String()
	}

	service := NewService(NewPostgresRepository(db), NewTierPostgresRepository(db))
	latitude, longitude := -15.3875, 28.3228

	t.Run("creates vendor and first store together", func(t *testing.T) {
		ownerID := createVendorUser(t)
		vendorRecord, err := service.OnboardVendorWithFirstStore(ctx, ownerID, "Atomic Print Studio", "", FirstStoreInput{
			Name:      "Main Branch",
			Address:   "123 Test Road",
			City:      "Lusaka",
			Country:   "Zambia",
			Latitude:  &latitude,
			Longitude: &longitude,
		})
		if err != nil {
			t.Fatalf("atomic onboarding: %v", err)
		}
		if vendorRecord.FirstStore == nil || vendorRecord.FirstStore.VendorID != vendorRecord.ID {
			t.Fatalf("expected a persisted first store owned by the new vendor")
		}

		var storeCount int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM stores WHERE vendor_id = $1`, vendorRecord.ID).Scan(&storeCount); err != nil {
			t.Fatalf("count first stores: %v", err)
		}
		if storeCount != 1 {
			t.Fatalf("expected exactly one first store, got %d", storeCount)
		}
	})

	t.Run("rolls back vendor when the first store cannot be inserted", func(t *testing.T) {
		ownerID := createVendorUser(t)
		_, err := service.OnboardVendorWithFirstStore(ctx, ownerID, "Rollback Print Studio", "", FirstStoreInput{
			Name:    "Invalid Branch",
			Address: "456 Test Road",
			City:    "Lusaka",
			Country: strings.Repeat("Z", 101),
		})
		if err == nil {
			t.Fatal("expected first-store insertion to fail for an overlong country")
		}

		var vendorCount int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vendors WHERE owner_id = $1`, ownerID).Scan(&vendorCount); err != nil {
			t.Fatalf("count vendors after rollback: %v", err)
		}
		if vendorCount != 0 {
			t.Fatalf("expected no vendor after failed atomic onboarding, got %d", vendorCount)
		}
	})
}
