CREATE TABLE unauthorized_attempts (
    id SERIAL PRIMARY KEY,
    phone_number TEXT NOT NULL UNIQUE,
    push_name TEXT,
    last_attempt TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
