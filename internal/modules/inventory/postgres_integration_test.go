package inventory

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/georgemunganga/printa-backend/internal/modules/vendor"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// TestListStoresByVendorHandlesNullableOptionalFields guards against a real
// portal failure where a correctly created store with a NULL description could
// not be listed because database/sql cannot scan NULL into a Go string.
func TestListStoresByVendorHandlesNullableOptionalFields(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}

	ctx := context.Background()
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	ownerID := uuid.New()
	email := "nullable-store-list-" + ownerID.String() + "@example.invalid"
	if _, err := db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, first_name, last_name, role, is_active)
		VALUES ($1, $2, $3, $4, $5, 'VENDOR', TRUE)
	`, ownerID, email, "not-used", "Nullable", "Store"); err != nil {
		t.Fatalf("create temporary user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, ownerID); err != nil {
			t.Errorf("clean up temporary user: %v", err)
		}
	})

	vendorService := vendor.NewService(vendor.NewPostgresRepository(db), vendor.NewTierPostgresRepository(db))
	vendorRecord, err := vendorService.OnboardVendorWithFirstStore(ctx, ownerID.String(), "Nullable Store Test", "", vendor.FirstStoreInput{
		Name: "No Description Branch", Address: "1 Test Road", City: "Lusaka", Country: "Zambia",
	})
	if err != nil {
		t.Fatalf("create vendor with first store: %v", err)
	}

	stores, err := NewStorePostgresRepository(db).ListStoresByVendor(ctx, vendorRecord.ID.String())
	if err != nil {
		t.Fatalf("list stores with nullable description: %v", err)
	}
	if len(stores) != 1 {
		t.Fatalf("expected one store, got %d", len(stores))
	}
	if stores[0].Description != "" {
		t.Fatalf("expected NULL description to normalize to empty string, got %q", stores[0].Description)
	}
}
