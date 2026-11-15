-- Valores de ENUM não podem ser removidos no PostgreSQL; os status novos
-- permanecem. Recria a tabela de peças (vazia) para o schema anterior.
CREATE TABLE IF NOT EXISTS parts (
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
