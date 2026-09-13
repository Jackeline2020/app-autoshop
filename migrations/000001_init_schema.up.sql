CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TYPE order_status AS ENUM (
    'recebida',
    'em_diagnostico',
    'aguardando_aprovacao',
    'em_execucao',
    'finalizada',
    'entregue',
    'recusada'
);

CREATE TABLE customers (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(255) NOT NULL,
    cpf                 VARCHAR(14),
    cnpj                VARCHAR(18),
    email               VARCHAR(255) NOT NULL,
    phone               VARCHAR(20) NOT NULL,
    address_street      VARCHAR(255) NOT NULL,
    address_number      VARCHAR(20) NOT NULL,
    address_complement  VARCHAR(255),
    address_city        VARCHAR(120) NOT NULL,
    address_state       VARCHAR(2) NOT NULL,
    address_zip_code    VARCHAR(10) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_customers_document CHECK (cpf IS NOT NULL OR cnpj IS NOT NULL)
);

CREATE UNIQUE INDEX uq_customers_cpf  ON customers (cpf)  WHERE cpf  IS NOT NULL;
CREATE UNIQUE INDEX uq_customers_cnpj ON customers (cnpj) WHERE cnpj IS NOT NULL;

CREATE TABLE vehicles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    plate       VARCHAR(8) NOT NULL UNIQUE,
    brand       VARCHAR(120) NOT NULL,
    model       VARCHAR(120) NOT NULL,
    year        INT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_vehicles_customer_id ON vehicles (customer_id);

CREATE TABLE services (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                    VARCHAR(255) NOT NULL,
    description             TEXT,
    price                   NUMERIC(10,2) NOT NULL,
    estimated_time_minutes  INT NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE parts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    price       NUMERIC(10,2) NOT NULL,
    stock       INT NOT NULL DEFAULT 0,
    min_stock   INT NOT NULL DEFAULT 0,
    unit        VARCHAR(20) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_parts_low_stock ON parts (stock, min_stock);

CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id     UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    customer_name   VARCHAR(255) NOT NULL,
    vehicle_id      UUID NOT NULL REFERENCES vehicles(id) ON DELETE RESTRICT,
    vehicle_plate   VARCHAR(8) NOT NULL,
    status          order_status NOT NULL DEFAULT 'recebida',
    total_services  NUMERIC(10,2) NOT NULL DEFAULT 0,
    total_parts     NUMERIC(10,2) NOT NULL DEFAULT 0,
    total           NUMERIC(10,2) NOT NULL DEFAULT 0,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ
);

CREATE INDEX idx_orders_customer_id ON orders (customer_id);
CREATE INDEX idx_orders_status ON orders (status);

CREATE TABLE order_services (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id      UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    service_id    UUID NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    service_name  VARCHAR(255) NOT NULL,
    price         NUMERIC(10,2) NOT NULL,
    executed_at   TIMESTAMPTZ
);

CREATE INDEX idx_order_services_order_id ON order_services (order_id);

CREATE TABLE order_parts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    part_id     UUID NOT NULL REFERENCES parts(id) ON DELETE RESTRICT,
    part_name   VARCHAR(255) NOT NULL,
    quantity    INT NOT NULL CHECK (quantity > 0),
    unit_price  NUMERIC(10,2) NOT NULL
);

CREATE INDEX idx_order_parts_order_id ON order_parts (order_id);

CREATE TABLE users (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           VARCHAR(255) NOT NULL,
    email          VARCHAR(255) NOT NULL UNIQUE,
    password_hash  VARCHAR(255) NOT NULL,
    role           VARCHAR(30) NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
