UPDATE vendor_tiers
SET
  features = CASE UPPER(name)
    WHEN 'CORE' THEN jsonb_set(jsonb_set(features, '{display_order}', '1'::jsonb, true), '{is_available}', 'true'::jsonb, true)
    WHEN 'PRO' THEN jsonb_set(jsonb_set(features, '{display_order}', '2'::jsonb, true), '{is_available}', 'false'::jsonb, true)
    WHEN 'ENTERPRISE' THEN jsonb_set(jsonb_set(features, '{display_order}', '3'::jsonb, true), '{is_available}', 'false'::jsonb, true)
    ELSE features
  END,
  updated_at = CURRENT_TIMESTAMP;
