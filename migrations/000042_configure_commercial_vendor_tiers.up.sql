-- The vendor portal offers two commercial monthly packages.
UPDATE vendor_tiers
SET
  monthly_price = CASE UPPER(name)
    WHEN 'PRO' THEN 500.00
    WHEN 'ENTERPRISE' THEN 1500.00
    ELSE monthly_price
  END,
  features = CASE UPPER(name)
    WHEN 'PRO' THEN jsonb_set(
      jsonb_set(
        jsonb_set(features, '{description}', '"For growing print shops and retail counters"'::jsonb, true),
        '{display_order}', '1'::jsonb, true
      ),
      '{is_available}', 'true'::jsonb, true
    )
    WHEN 'ENTERPRISE' THEN jsonb_set(
      jsonb_set(
        jsonb_set(features, '{description}', '"For high-volume and multi-store operations"'::jsonb, true),
        '{display_order}', '2'::jsonb, true
      ),
      '{is_available}', 'true'::jsonb, true
    )
    ELSE jsonb_set(
      jsonb_set(features, '{display_order}', '999'::jsonb, true),
      '{is_available}', 'false'::jsonb, true
    )
  END,
  updated_at = CURRENT_TIMESTAMP;
