package db

import (
	"database/sql"
	"fmt"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) IsPhoneAllowed(phone string) (bool, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM allowed_phones WHERE phone_number = $1", phone).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check phone: %w", err)
	}
	return count > 0, nil
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
	rows, err := r.db.Query("SELECT id, phone_number, description, created_at FROM allowed_phones ORDER BY id DESC")
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
	return phones, nil
}

func (r *Repository) AddAllowedPhone(phone string, description string) error {
	_, err := r.db.Exec("INSERT INTO allowed_phones (phone_number, description) VALUES ($1, $2)", phone, description)
	if err != nil {
		return fmt.Errorf("failed to add phone: %w", err)
	}
	return nil
}

func (r *Repository) DeleteAllowedPhone(id int) error {
	_, err := r.db.Exec("DELETE FROM allowed_phones WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete phone: %w", err)
	}
	return nil
}

func (r *Repository) ListProcessedMedia(phoneFilter string) ([]ProcessedMedia, error) {
	query := `
		SELECT pm.id, pm.media_type, pm.file_path, pm.extracted_text, pm.sender_phone, pm.status, pm.error_message, pm.created_at, ap.description 
		FROM processed_media pm
		LEFT JOIN allowed_phones ap ON pm.sender_phone = ap.phone_number
	`
	var args []interface{}
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
		var errMsg sql.NullString
		var extText sql.NullString
		var name sql.NullString
		if err := rows.Scan(&m.ID, &m.MediaType, &m.FilePath, &extText, &m.SenderPhone, &m.Status, &errMsg, &m.CreatedAt, &name); err != nil {
			return nil, err
		}
		if errMsg.Valid {
			m.ErrorMessage = errMsg.String
		}
		if extText.Valid {
			m.ExtractedText = extText.String
		}
		if name.Valid {
			m.SenderName = name.String
		}
		media = append(media, m)
	}
	return media, nil
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

func (r *Repository) DeleteProcessedMedia(id int) (string, error) {
	var filePath string
	err := r.db.QueryRow("DELETE FROM processed_media WHERE id = $1 RETURNING file_path", id).Scan(&filePath)
	if err != nil {
		return "", fmt.Errorf("failed to delete processed media: %w", err)
	}
	return filePath, nil
}

func (r *Repository) SaveUnauthorizedAttempt(phone, name string) (bool, error) {
	var lastAttempt sql.NullTime
	err := r.db.QueryRow("SELECT last_attempt FROM unauthorized_attempts WHERE phone_number = $1", phone).Scan(&lastAttempt)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("failed to check last attempt: %w", err)
	}

	needsNotification := false
	if !lastAttempt.Valid || time.Since(lastAttempt.Time) > 1*time.Hour {
		needsNotification = true
	}

	query := `
		INSERT INTO unauthorized_attempts (phone_number, push_name, last_attempt)
		VALUES ($1, $2, NOW())
		ON CONFLICT (phone_number) DO UPDATE SET
			push_name = EXCLUDED.push_name,
			last_attempt = EXCLUDED.last_attempt
	`
	_, err = r.db.Exec(query, phone, name)
	if err != nil {
		return false, fmt.Errorf("failed to save unauthorized attempt: %w", err)
	}
	return needsNotification, nil
}

func (r *Repository) ListUnauthorizedAttempts() ([]UnauthorizedAttempt, error) {
	rows, err := r.db.Query("SELECT id, phone_number, push_name, last_attempt FROM unauthorized_attempts ORDER BY last_attempt DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to list unauthorized attempts: %w", err)
	}
	defer rows.Close()

	var attempts []UnauthorizedAttempt
	for rows.Next() {
		var a UnauthorizedAttempt
		var name sql.NullString
		if err := rows.Scan(&a.ID, &a.PhoneNumber, &name, &a.LastAttempt); err != nil {
			return nil, err
		}
		if name.Valid {
			a.PushName = name.String
		}
		attempts = append(attempts, a)
	}
	return attempts, nil
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
		SELECT DISTINCT pm.sender_phone, ap.description 
		FROM processed_media pm
		LEFT JOIN allowed_phones ap ON pm.sender_phone = ap.phone_number
		ORDER BY ap.description, pm.sender_phone
	`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list media users: %w", err)
	}
	defer rows.Close()

	var users []MediaUser
	for rows.Next() {
		var u MediaUser
		var name sql.NullString
		if err := rows.Scan(&u.PhoneNumber, &name); err != nil {
			return nil, err
		}
		if name.Valid {
			u.Name = name.String
		}
		users = append(users, u)
	}
	return users, nil
}
