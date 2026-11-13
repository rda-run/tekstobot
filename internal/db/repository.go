package db

import (
	"database/sql"
	"fmt"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) IsPhoneAllowed(phone string) (bool, error) {
	var allowed bool
	err := r.db.QueryRow("SELECT EXISTS(SELECT 1 FROM allowed_phones WHERE phone_number = $1)", phone).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("failed to check phone: %w", err)
	}
	return allowed, nil
}

func (r *Repository) SaveProcessedMedia(media *ProcessedMedia) (int, error) {
	var id int
	query := `
		INSERT INTO processed_media (media_type, file_path, extracted_text, sender_phone, status, error_message)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	err := r.db.QueryRow(query, media.MediaType, media.FilePath, media.ExtractedText, media.SenderPhone, media.Status, media.ErrorMessage).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert processed media: %w", err)
	}
	return id, nil
}

func (r *Repository) ListAllowedPhones() ([]AllowedPhone, error) {
	rows, err := r.db.Query("SELECT id, phone_number, COALESCE(description, ''), created_at FROM allowed_phones ORDER BY id DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to list phones: %w", err)
	}
	defer rows.Close()

	var phones []AllowedPhone
	for rows.Next() {
		var p AllowedPhone
		if err := rows.Scan(&p.ID, &p.PhoneNumber, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		phones = append(phones, p)
	}
	return phones, rows.Err()
}

func (r *Repository) AddAllowedPhone(phone string, description string) error {
	_, err := r.db.Exec("INSERT INTO allowed_phones (phone_number, description) VALUES ($1, $2)", phone, description)
	if err != nil {
		return fmt.Errorf("failed to add phone: %w", err)
	}
	return nil
}

// DeleteAllowedPhone remove o telefone e devolve os arquivos das mídias que
// o ON DELETE CASCADE apaga junto, para que o chamador os remova do disco.
func (r *Repository) DeleteAllowedPhone(id int) ([]string, error) {
	// O SELECT enxerga o snapshot anterior ao DELETE; o cascade roda ao fim do statement.
	rows, err := r.db.Query(`
		WITH del AS (DELETE FROM allowed_phones WHERE id = $1 RETURNING phone_number)
		SELECT pm.file_path FROM processed_media pm JOIN del ON pm.sender_phone = del.phone_number
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to delete phone: %w", err)
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return files, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

func (r *Repository) ListProcessedMedia(phoneFilter string) ([]ProcessedMedia, error) {
	query := `
		SELECT pm.id, pm.media_type, pm.file_path, COALESCE(pm.extracted_text, ''), pm.sender_phone, pm.status,
		       COALESCE(pm.error_message, ''), pm.created_at, COALESCE(ap.description, '')
		FROM processed_media pm
		LEFT JOIN allowed_phones ap ON pm.sender_phone = ap.phone_number
	`
	var args []any
	if phoneFilter != "" {
		query += " WHERE pm.sender_phone = $1"
		args = append(args, phoneFilter)
	}
	query += " ORDER BY pm.id DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list processed media: %w", err)
	}
	defer rows.Close()

	var media []ProcessedMedia
	for rows.Next() {
		var m ProcessedMedia
		if err := rows.Scan(&m.ID, &m.MediaType, &m.FilePath, &m.ExtractedText, &m.SenderPhone, &m.Status, &m.ErrorMessage, &m.CreatedAt, &m.SenderName); err != nil {
			return nil, err
		}
		media = append(media, m)
	}
	return media, rows.Err()
}

func (r *Repository) UpdateProcessedMedia(id int, extractedText, status, errorMessage string) error {
	query := `
		UPDATE processed_media
		SET extracted_text = $1, status = $2, error_message = $3
		WHERE id = $4
	`
	_, err := r.db.Exec(query, extractedText, status, errorMessage, id)
	if err != nil {
		return fmt.Errorf("failed to update processed media: %w", err)
	}
	return nil
}

// FailStalePending marca como erro as mídias que ficaram "pending" porque o
// processo foi encerrado no meio da transcrição. Chamar apenas na inicialização.
func (r *Repository) FailStalePending() (int64, error) {
	res, err := r.db.Exec(`
		UPDATE processed_media
		SET status = 'error', error_message = 'Transcription interrupted by a service restart.'
		WHERE status = 'pending'
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to fail stale pending media: %w", err)
	}
	return res.RowsAffected()
}

func (r *Repository) DeleteProcessedMedia(id int) (string, error) {
	var filePath string
	err := r.db.QueryRow("DELETE FROM processed_media WHERE id = $1 RETURNING file_path", id).Scan(&filePath)
	if err != nil {
		return "", fmt.Errorf("failed to delete processed media: %w", err)
	}
	return filePath, nil
}

// SaveUnauthorizedAttempt registra a tentativa e informa se é preciso notificar
// (primeira tentativa, ou última há mais de 1 hora). Um único statement atômico,
// usando apenas o relógio do banco.
func (r *Repository) SaveUnauthorizedAttempt(phone, name string) (bool, error) {
	var needsNotification bool
	query := `
		WITH prev AS (SELECT last_attempt FROM unauthorized_attempts WHERE phone_number = $1)
		INSERT INTO unauthorized_attempts (phone_number, push_name, last_attempt)
		VALUES ($1, $2, NOW())
		ON CONFLICT (phone_number) DO UPDATE SET
			push_name = EXCLUDED.push_name,
			last_attempt = EXCLUDED.last_attempt
		RETURNING COALESCE((SELECT last_attempt < NOW() - INTERVAL '1 hour' FROM prev), TRUE)
	`
	if err := r.db.QueryRow(query, phone, name).Scan(&needsNotification); err != nil {
		return false, fmt.Errorf("failed to save unauthorized attempt: %w", err)
	}
	return needsNotification, nil
}

func (r *Repository) ListUnauthorizedAttempts() ([]UnauthorizedAttempt, error) {
	rows, err := r.db.Query("SELECT id, phone_number, COALESCE(push_name, ''), last_attempt FROM unauthorized_attempts ORDER BY last_attempt DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to list unauthorized attempts: %w", err)
	}
	defer rows.Close()

	var attempts []UnauthorizedAttempt
	for rows.Next() {
		var a UnauthorizedAttempt
		if err := rows.Scan(&a.ID, &a.PhoneNumber, &a.PushName, &a.LastAttempt); err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

func (r *Repository) DeleteUnauthorizedAttempt(phone string) error {
	_, err := r.db.Exec("DELETE FROM unauthorized_attempts WHERE phone_number = $1", phone)
	if err != nil {
		return fmt.Errorf("failed to delete unauthorized attempt: %w", err)
	}
	return nil
}

func (r *Repository) ListMediaUsers() ([]MediaUser, error) {
	query := `
		SELECT DISTINCT pm.sender_phone, COALESCE(ap.description, '') AS name
		FROM processed_media pm
		LEFT JOIN allowed_phones ap ON pm.sender_phone = ap.phone_number
		ORDER BY name, pm.sender_phone
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list media users: %w", err)
	}
	defer rows.Close()

	var users []MediaUser
	for rows.Next() {
		var u MediaUser
		if err := rows.Scan(&u.PhoneNumber, &u.Name); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
