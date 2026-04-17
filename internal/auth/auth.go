package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"tekstobot/internal/config"
)

// Session representa uma sessão ativa no banco.
type Session struct {
	ID          string
	Email       string
	DisplayName string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// Auth encapsula o client OIDC e operações de sessão.
type Auth struct {
	cfg        *config.Config
	db         *sql.DB
	provider   *oidc.Provider
	oauth2Cfg  oauth2.Config
	verifier   *oidc.IDTokenVerifier
	sessionTTL time.Duration
}

// New inicializa o provider OIDC via discovery.
func New(cfg *config.Config, db *sql.DB) (*Auth, error) {
	ctx := context.Background()

	provider, err := oidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc provider discovery failed: %w", err)
	}

	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.OIDCClientID,
	})

	return &Auth{
		cfg:        cfg,
		db:         db,
		provider:   provider,
		oauth2Cfg:  oauth2Cfg,
		verifier:   verifier,
		sessionTTL: time.Duration(cfg.OIDCSessionTTL) * time.Hour,
	}, nil
}

// ---------- Session management ----------

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateSession persiste uma nova sessão no banco e retorna o ID.
func (a *Auth) CreateSession(email, displayName string) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", fmt.Errorf("failed to generate session id: %w", err)
	}

	expiresAt := time.Now().Add(a.sessionTTL)

	_, err = a.db.Exec(
		`INSERT INTO sessions (id, email, display_name, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		id, email, displayName, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return id, nil
}

// GetSession busca uma sessão válida (não expirada) pelo ID.
func (a *Auth) GetSession(id string) (*Session, error) {
	var s Session
	err := a.db.QueryRow(
		`SELECT id, email, display_name, created_at, expires_at
		 FROM sessions WHERE id = $1 AND expires_at > NOW()`,
		id,
	).Scan(&s.ID, &s.Email, &s.DisplayName, &s.CreatedAt, &s.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// DeleteSession remove uma sessão pelo ID.
func (a *Auth) DeleteSession(id string) error {
	_, err := a.db.Exec("DELETE FROM sessions WHERE id = $1", id)
	return err
}

// CleanExpiredSessions remove todas as sessões expiradas.
func (a *Auth) CleanExpiredSessions() {
	result, err := a.db.Exec("DELETE FROM sessions WHERE expires_at < NOW()")
	if err != nil {
		log.Printf("Failed to clean expired sessions: %v", err)
		return
	}
	if n, _ := result.RowsAffected(); n > 0 {
		log.Printf("Cleaned %d expired sessions", n)
	}
}

// ---------- HTTP Handlers ----------

// HandleLogin redireciona o usuário para o Pocket ID.
func (a *Auth) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateSessionID()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Salvar state em cookie temporário para validar no callback
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutos
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(a.cfg.OIDCIssuerURL, "https"),
	})

	http.Redirect(w, r, a.oauth2Cfg.AuthCodeURL(state), http.StatusFound)
}

// HandleCallback processa o retorno do Pocket ID.
func (a *Auth) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// 1. Validar state
	stateCookie, err := r.Cookie("oidc_state")
	if err != nil || stateCookie.Value == "" {
		http.Error(w, "Missing state cookie", http.StatusBadRequest)
		return
	}

	if r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Limpar cookie de state
	http.SetCookie(w, &http.Cookie{
		Name:   "oidc_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// 2. Trocar code por token
	ctx := r.Context()
	oauth2Token, err := a.oauth2Cfg.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		log.Printf("OIDC token exchange failed: %v", err)
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	// 3. Extrair e verificar id_token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token in response", http.StatusUnauthorized)
		return
	}

	idToken, err := a.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		log.Printf("OIDC id_token verification failed: %v", err)
		http.Error(w, "Token verification failed", http.StatusUnauthorized)
		return
	}

	// 4. Extrair claims do usuário
	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		log.Printf("Failed to parse claims: %v", err)
		http.Error(w, "Failed to parse user info", http.StatusInternalServerError)
		return
	}

	// 5. Criar sessão no banco
	sessionID, err := a.CreateSession(claims.Email, claims.Name)
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// 6. Definir cookie de sessão
	http.SetCookie(w, &http.Cookie{
		Name:     "tekstobot_session",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   a.cfg.OIDCSessionTTL * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(a.cfg.OIDCIssuerURL, "https"),
	})

	log.Printf("OIDC login successful: %s (%s)", claims.Email, claims.Name)
	http.Redirect(w, r, "/", http.StatusFound)
}

// HandleLogout destrói a sessão e redireciona para login.
func (a *Auth) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("tekstobot_session")
	if err == nil {
		a.DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "tekstobot_session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	http.Redirect(w, r, "/auth/login", http.StatusFound)
}

// ---------- Middleware ----------

// RequireAuth é um middleware que protege rotas.
// Redireciona para /auth/login se não houver sessão válida.
func (a *Auth) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("tekstobot_session")
		if err != nil {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		session, err := a.GetSession(cookie.Value)
		if err != nil {
			// Sessão inválida ou expirada
			http.SetCookie(w, &http.Cookie{
				Name:   "tekstobot_session",
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		// Injetar sessão no contexto do request
		ctx := context.WithValue(r.Context(), contextKeySession, session)
		next(w, r.WithContext(ctx))
	}
}

// RequireAuthHandler wraps an http.Handler with authentication.
func (a *Auth) RequireAuthHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("tekstobot_session")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		_, err = a.GetSession(cookie.Value)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// contextKey tipo privado para chaves de contexto.
type contextKey string

const contextKeySession contextKey = "session"

// SessionFromContext extrai a sessão do contexto do request.
func SessionFromContext(ctx context.Context) *Session {
	s, _ := ctx.Value(contextKeySession).(*Session)
	return s
}
