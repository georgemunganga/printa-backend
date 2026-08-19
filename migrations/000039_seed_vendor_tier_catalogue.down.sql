-- Do not remove tiers that are referenced by a subscription during rollback.
UPDATE vendor_tiers
SET
  monthly_price = 0.00,
  features = '{"description":"Default platform tier. Pricing and feature limits may be updated by platform administration."}'::jsonb,
  updated_at = CURRENT_TIMESTAMP
WHERE name = 'Core';

UPDATE vendor_tiers
SET name = 'CORE', updated_at = CURRENT_TIMESTAMP
WHERE name = 'Core';

DELETE FROM vendor_tiers vt
WHERE vt.name IN ('Pro', 'Enterprise')
  AND NOT EXISTS (
    SELECT 1
    FROM vendor_subscriptions vs
    WHERE vs.tier_id = vt.id
  );
