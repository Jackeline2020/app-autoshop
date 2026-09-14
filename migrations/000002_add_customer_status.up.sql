ALTER TABLE customers
    ADD COLUMN status VARCHAR(10) NOT NULL DEFAULT 'ativo'
        CONSTRAINT chk_customers_status CHECK (status IN ('ativo', 'inativo'));

CREATE INDEX idx_customers_status ON customers (status);
