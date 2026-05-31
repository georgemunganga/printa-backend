-- Add role enum type
DO $$ BEGIN
  CREATE TYPE user_role AS ENUM ('ADMIN','VENDOR','STAFF','CASHIER','CUSTOMER');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Add role column with default CUSTOMER
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS role user_role NOT NULL DEFAULT 'CUSTOMER',
  ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN IF NOT EXISTS phone VARCHAR(20);

-- Index for role-based queries
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
