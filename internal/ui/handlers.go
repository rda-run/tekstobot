package ui

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/skip2/go-qrcode"

	"tekstobot/internal/auth"
	"tekstobot/internal/db"
	"tekstobot/internal/whatsapp"
	"tekstobot/web"
)

var nonDigits = regexp.MustCompile(`\D`)

type Server struct {
	Repo           *db.Repository
	WA             *whatsapp.Client
	Version        string
	MigrationError string
	Auth           *auth.Auth // nil when OIDC is disabled
}

func NewServer(repo *db.Repository, wa *whatsapp.Client, version string, migrationError string, authHandler *auth.Auth) *Server {
	return &Server{
		Repo:           repo,
		WA:             wa,
		Version:        version,
		MigrationError: migrationError,
		Auth:           authHandler,
	}
}

func (s *Server) Start(port string) error {
	mux := http.NewServeMux()

	// protect wraps a handler with auth middleware when OIDC is enabled.
	protect := func(handler http.HandlerFunc) http.HandlerFunc {
		if s.Auth != nil {
			return s.Auth.RequireAuth(handler)
		}
		return handler
	}

	// Auth routes (always public)
	if s.Auth != nil {
		mux.HandleFunc("/auth/login", s.Auth.HandleLogin)
		mux.HandleFunc("/auth/callback", s.Auth.HandleCallback)
		mux.HandleFunc("/auth/logout", s.Auth.HandleLogout)
	}

	// API JSON usada pela interface (web/). Tudo que altera estado exige POST/DELETE: o cookie
	// de sessão é SameSite=Lax, então um link GET em outro site conseguiria, por exemplo,
	// autorizar um número.
	mux.HandleFunc("GET /api/info", protect(s.handleInfo))
	mux.HandleFunc("GET /api/status", protect(s.handleStatus))
	mux.HandleFunc("POST /api/whatsapp/reset", protect(s.handleWhatsAppReset))
	mux.HandleFunc("GET /qr.png", protect(s.handleQR))
	mux.HandleFunc("GET /api/phones", protect(s.handlePhones))
	mux.HandleFunc("POST /api/phones", protect(s.handleAddPhone))
	mux.HandleFunc("DELETE /api/phones/{id}", protect(s.handleDeletePhone))
	mux.HandleFunc("GET /api/media", protect(s.handleMedia))
	mux.HandleFunc("DELETE /api/media/{id}", protect(s.handleDeleteMedia))
	mux.HandleFunc("POST /api/unauthorized/authorize", protect(s.handleAuthorizeUnauthorized))
	mux.HandleFunc("DELETE /api/unauthorized/{phone}", protect(s.handleDeleteUnauthorized))

	// Media file server (protected when OIDC is enabled)
	mediaFS := http.StripPrefix("/data/media/", http.FileServer(http.Dir("data/media")))
	mux.HandleFunc("GET /data/media/", protect(mediaFS.ServeHTTP))

	// Interface (build do Vite embutido no binário).
	mux.HandleFunc("GET /", protect(http.FileServerFS(web.Files()).ServeHTTP))

	log.Printf("Starting UI Server on port %s", port)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

// writeJSON responde com v em JSON, registrando falhas no log.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	logErr("write json", json.NewEncoder(w).Encode(v))
}

// list troca nil por lista vazia: no JSON, nil viraria null e a interface espera [].
func list[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

// logErr registra erros que a UI não tem como exibir, em vez de engoli-los.
func logErr(action string, err error) {
	if err != nil {
		log.Printf("UI: %s: %v", action, err)
	}
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	var userName string
	if session := auth.SessionFromContext(r.Context()); session != nil {
		userName = session.DisplayName
	}
	writeJSON(w, map[string]any{
		"version":        s.Version,
		"migrationError": s.MigrationError,
		"userName":       userName,
		"authEnabled":    s.Auth != nil,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	attempts, err := s.Repo.ListUnauthorizedAttempts()
	logErr("list unauthorized attempts", err)
	writeJSON(w, map[string]any{
		"connected": s.WA.WAClient.IsConnected() && s.WA.WAClient.IsLoggedIn(),
		"hasQR":     s.WA.GetQR() != "",
		"attempts":  list(attempts),
	})
}

func (s *Server) handleWhatsAppReset(w http.ResponseWriter, r *http.Request) {
	logErr("reset whatsapp session", s.WA.ResetSession())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleQR(w http.ResponseWriter, r *http.Request) {
	qrCode := s.WA.GetQR()
	if qrCode == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	png, err := qrcode.Encode(qrCode, qrcode.Medium, 256)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(png)
}

func (s *Server) handlePhones(w http.ResponseWriter, r *http.Request) {
	phones, err := s.Repo.ListAllowedPhones()
	logErr("list allowed phones", err)
	writeJSON(w, list(phones))
}

func (s *Server) handleAddPhone(w http.ResponseWriter, r *http.Request) {
	var body struct{ Phone, Description string }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	// O WhatsApp identifica o remetente só por dígitos: "+55 (11) 9..." nunca casaria.
	phone := nonDigits.ReplaceAllString(body.Phone, "")
	if phone == "" {
		http.Error(w, "phone number is required", http.StatusBadRequest)
		return
	}
	logErr("add phone", s.Repo.AddAllowedPhone(phone, body.Description))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeletePhone(w http.ResponseWriter, r *http.Request) {
	if id, _ := strconv.Atoi(r.PathValue("id")); id != 0 {
		files, err := s.Repo.DeleteAllowedPhone(id)
		logErr("delete phone", err)
		for _, f := range files {
			os.Remove(f)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	media, err := s.Repo.ListProcessedMedia(r.URL.Query().Get("phone"))
	logErr("list processed media", err)
	users, err := s.Repo.ListMediaUsers()
	logErr("list media users", err)
	writeJSON(w, map[string]any{"items": list(media), "users": list(users)})
}

func (s *Server) handleDeleteMedia(w http.ResponseWriter, r *http.Request) {
	if id, _ := strconv.Atoi(r.PathValue("id")); id != 0 {
		filePath, err := s.Repo.DeleteProcessedMedia(id)
		logErr("delete media", err)
		if err == nil && filePath != "" {
			os.Remove(filePath)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAuthorizeUnauthorized(w http.ResponseWriter, r *http.Request) {
	var body struct{ Phone, Name string }
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || body.Phone == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	logErr("authorize phone", s.Repo.AddAllowedPhone(body.Phone, body.Name))
	logErr("delete unauthorized attempt", s.Repo.DeleteUnauthorizedAttempt(body.Phone))
	s.WA.Notify(body.Phone, "Seu acesso no TekstoBot foi autorizado! 🎉\nPor favor, reenvie o seu áudio ou mensagem para que eu possa processar.")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteUnauthorized(w http.ResponseWriter, r *http.Request) {
	logErr("delete unauthorized attempt", s.Repo.DeleteUnauthorizedAttempt(r.PathValue("phone")))
	w.WriteHeader(http.StatusNoContent)
}
