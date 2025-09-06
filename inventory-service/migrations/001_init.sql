-- create product and reservations
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS products (
  sku text PRIMARY KEY,
  name text,
  total_stock integer NOT NULL DEFAULT 0,
  reserved_stock integer NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS reservations (
  reservation_id uuid PRIMARY KEY,
  sku text NOT NULL REFERENCES products(sku),
  qty integer NOT NULL,
  status varchar(20) NOT NULL,
  created_at timestamptz,
  updated_at timestamptz
);

-- sample data
INSERT INTO products (sku, name, total_stock) VALUES ('sku-1', 'Sample Product 1', 100) ON CONFLICT (sku) DO NOTHING;
INSERT INTO products (sku, name, total_stock) VALUES ('sku-2', 'Sample Product 2', 50) ON CONFLICT (sku) DO NOTHING;
