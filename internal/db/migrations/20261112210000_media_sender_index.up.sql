-- Índice para o filtro por usuário no histórico e para o ON DELETE CASCADE de allowed_phones.
CREATE INDEX IF NOT EXISTS idx_processed_media_sender_phone ON processed_media (sender_phone);
