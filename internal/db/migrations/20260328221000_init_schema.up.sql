CREATE TABLE allowed_phones (
    id SERIAL PRIMARY KEY,
    phone_number VARCHAR(30) UNIQUE NOT NULL,
    description VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE processed_media (
    id SERIAL PRIMARY KEY,
    media_type VARCHAR(20) NOT NULL, -- 'audio', 'image'
    file_path VARCHAR(255) NOT NULL, -- Local no disco onde a mídia ficará salva
    extracted_text TEXT,
    sender_phone VARCHAR(30) NOT NULL REFERENCES allowed_phones(phone_number) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'processing', -- 'processing', 'completed', 'error'
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
