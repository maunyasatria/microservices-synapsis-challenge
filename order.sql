Create Table order_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id uuid REFERENCES ordes(id) ON DELETE CASCADE,
    sku text NOT NULL,
    qty int NOT NULL,
    unit_price bigint,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP
)