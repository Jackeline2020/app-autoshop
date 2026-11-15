-- Peças e estoque passam a pertencer ao Execution Service (Fase 4). A OS
-- guarda apenas uma cópia (nome, preço e quantidade) das peças usadas.
ALTER TABLE order_parts DROP CONSTRAINT IF EXISTS order_parts_part_id_fkey;
ALTER TABLE order_parts ALTER COLUMN part_id TYPE VARCHAR(64) USING part_id::text;
DROP TABLE IF EXISTS parts;

-- Status controlados pela Saga
ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'aguardando_pagamento';
ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'aguardando_execucao';
ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'cancelada';
