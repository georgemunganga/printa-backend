INSERT INTO vendor_tiers (name, monthly_price, features)
VALUES (
    'CORE',
    0.00,
    '{"description":"Default platform tier. Pricing and feature limits may be updated by platform administration."}'::jsonb
)
ON CONFLICT (name) DO NOTHING;
