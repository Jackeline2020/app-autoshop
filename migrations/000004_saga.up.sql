-- Dados da OS preenchidos ao longo da Saga. diagnosis_at e
-- waiting_approval_at já existiam no domínio, mas não eram gravados.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS diagnosis TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS diagnosis_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS waiting_approval_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS checkout_url TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS cancellation_reason TEXT;

-- Estado de cada Saga (uma por OS)
CREATE TABLE saga_instances (
    id                     UUID PRIMARY KEY,
    order_id               UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    status                 VARCHAR(20) NOT NULL CHECK (status IN ('em_andamento', 'compensando', 'concluida', 'cancelada')),
    current_step           VARCHAR(30) NOT NULL,
    completed_steps        TEXT[] NOT NULL DEFAULT '{}',
    pending_compensations  TEXT[] NOT NULL DEFAULT '{}',
    deadline_at            TIMESTAMPTZ,
    correlation_id         VARCHAR(64),
    failure_reason         TEXT,
    version                INT NOT NULL DEFAULT 1,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_saga_instances_deadline ON saga_instances (deadline_at) WHERE deadline_at IS NOT NULL;

-- Comandos gravados na mesma transação da Saga e publicados depois
CREATE TABLE outbox (
    id            UUID PRIMARY KEY,
    queue         VARCHAR(40) NOT NULL,
    message_type  VARCHAR(40) NOT NULL,
    body          JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX idx_outbox_pending ON outbox (created_at) WHERE published_at IS NULL;

-- Eventos já processados, para ignorar entregas repetidas do SQS
CREATE TABLE processed_messages (
    message_id    VARCHAR(64) PRIMARY KEY,
    processed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
