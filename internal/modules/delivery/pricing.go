package delivery

import (
	"context"
	"database/sql"
	"fmt"
	"math"
)

// PriceQuote is a server-calculated delivery charge from a store to a destination.
type PriceQuote struct {
	DistanceKM float64 `json:"distance_km"`
	Fee        float64 `json:"fee"`
	Currency   string  `json:"currency"`
	RuleID     string  `json:"rule_id"`
}

// PricingService resolves the active database pricing rule for a route.
type PricingService interface {
	Quote(ctx context.Context, storeID string, latitude, longitude float64) (*PriceQuote, error)
}

type pricingService struct{ db *sql.DB }

func NewPricingService(db *sql.DB) PricingService { return &pricingService{db: db} }

func (s *pricingService) Quote(ctx context.Context, storeID string, latitude, longitude float64) (*PriceQuote, error) {
	if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return nil, fmt.Errorf("delivery coordinates are invalid")
	}
	var storeLatitude, storeLongitude sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `SELECT latitude, longitude FROM stores WHERE id=$1 AND is_active=true`, storeID).Scan(&storeLatitude, &storeLongitude); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("store not found")
		}
		return nil, err
	}
	if !storeLatitude.Valid || !storeLongitude.Valid {
		return nil, fmt.Errorf("store coordinates are not configured")
	}

	distance := haversineKM(storeLatitude.Float64, storeLongitude.Float64, latitude, longitude)
	var ruleID, currency string
	var baseFee, perKMFee, perKMAbove float64
	if err := s.db.QueryRowContext(ctx, `
		SELECT id, base_fee, per_km_fee, per_km_above_km, currency
		FROM delivery_pricing_rules
		WHERE is_active=true AND (max_distance_km IS NULL OR max_distance_km >= $1)
		ORDER BY max_distance_km ASC NULLS LAST, sort_order ASC
		LIMIT 1`, distance).Scan(&ruleID, &baseFee, &perKMFee, &perKMAbove, &currency); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("delivery pricing is not configured")
		}
		return nil, err
	}

	fee := calculateRuleFee(baseFee, perKMFee, perKMAbove, distance)
	return &PriceQuote{DistanceKM: round2(distance), Fee: round2(fee), Currency: currency, RuleID: ruleID}, nil
}

func calculateRuleFee(baseFee, perKMFee, perKMAbove, distance float64) float64 {
	fee := baseFee
	if perKMFee > 0 && distance > perKMAbove {
		fee += math.Ceil(distance-perKMAbove) * perKMFee
	}
	return fee
}

func haversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKM = 6371.0
	toRadians := func(value float64) float64 { return value * math.Pi / 180 }
	dLat := toRadians(lat2 - lat1)
	dLon := toRadians(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(toRadians(lat1))*math.Cos(toRadians(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusKM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func round2(value float64) float64 { return math.Round(value*100) / 100 }
