-- Server-authoritative monthly catalogue for the vendor subscription experience.
-- The portal reads these records at runtime; future pricing and presentation updates are data changes.
UPDATE vendor_tiers
SET name = INITCAP(LOWER(name)), updated_at = CURRENT_TIMESTAMP
WHERE name IN ('CORE', 'PRO', 'ENTERPRISE');

INSERT INTO vendor_tiers AS vt (name, monthly_price, features)
VALUES
  (
    'Core',
    250.00,
    '{
      "description": "For getting started",
      "display_order": 1,
      "is_available": true,
      "is_popular": false,
      "features": [
        {"text": "20 jobs/day", "included": true},
        {"text": "3 team members", "included": true},
        {"text": "Standard routing", "included": true},
        {"text": "Basic reporting", "included": true},
        {"text": "Priority routing", "included": false},
        {"text": "SLA guarantees", "included": false}
      ]
    }'::jsonb
  ),
  (
    'Pro',
    500.00,
    '{
      "description": "Commercial configuration pending",
      "display_order": 2,
      "is_available": false,
      "is_popular": true,
      "features": [
        {"text": "100 jobs/day", "included": true},
        {"text": "10 team members", "included": true},
        {"text": "Priority routing", "included": true},
        {"text": "Advanced reporting", "included": true},
        {"text": "2-hour handoff SLA", "included": true},
        {"text": "Bulk price updates", "included": true}
      ]
    }'::jsonb
  ),
  (
    'Enterprise',
    1500.00,
    '{
      "description": "Commercial configuration pending",
      "display_order": 3,
      "is_available": false,
      "is_popular": false,
      "features": [
        {"text": "Unlimited jobs/day", "included": true},
        {"text": "Unlimited team", "included": true},
        {"text": "Highest priority", "included": true},
        {"text": "Custom SLA", "included": true},
        {"text": "Account manager", "included": true},
        {"text": "API access", "included": true}
      ]
    }'::jsonb
  )
ON CONFLICT (name) DO UPDATE
SET
  monthly_price = EXCLUDED.monthly_price,
  features = EXCLUDED.features,
  updated_at = CURRENT_TIMESTAMP;
