package ui

import (
	"embed"
	"html/template"
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
)

//go:embed templates/*.html
var templatesFS embed.FS

var nonDigits = regexp.MustCompile(`\D`)

type Server struct {
	Repo           *db.Repository
	WA             *whatsapp.Client
	Tpl            *template.Template
	Version        string
	MigrationError string
	Auth           *auth.Auth // nil when OIDC is disabled
}

func NewServer(repo *db.Repository, wa *whatsapp.Client, version string, migrationError string, authHandler *auth.Auth) *Server {
	tpl := template.Must(template.ParseFS(templatesFS, "templates/*.html"))
	return &Server{
		Repo:           repo,
		WA:             wa,
		Tpl:            tpl,
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

	// Protected routes. Tudo que altera estado exige POST/DELETE: o cookie de sessão é
	// SameSite=Lax, então um link GET em outro site conseguiria, por exemplo, autorizar um número.
	mux.HandleFunc("GET /{$}", protect(s.handleDashboard))
	mux.HandleFunc("GET /status", protect(s.handleStatus))
	mux.HandleFunc("POST /whatsapp/reset", protect(s.handleWhatsAppReset))
	mux.HandleFunc("GET /qr.png", protect(s.handleQR))
	mux.HandleFunc("GET /phones", protect(s.handlePhones))
	mux.HandleFunc("POST /phones/add", protect(s.handleAddPhone))
	mux.HandleFunc("DELETE /phones/delete", protect(s.handleDeletePhone))
	mux.HandleFunc("GET /media", protect(s.handleMedia))
	mux.HandleFunc("DELETE /media/delete", protect(s.handleDeleteMedia))
	mux.HandleFunc("GET /unauthorized", protect(s.handleListUnauthorized))
	mux.HandleFunc("POST /unauthorized/authorize", protect(s.handleAuthorizeUnauthorized))
	mux.HandleFunc("DELETE /unauthorized/delete", protect(s.handleDeleteUnauthorized))

	// Media file server (protected when OIDC is enabled)
	mediaFS := http.StripPrefix("/data/media/", http.FileServer(http.Dir("data/media")))
	mux.HandleFunc("GET /data/media/", protect(mediaFS.ServeHTTP))

	log.Printf("Starting UI Server on port %s", port)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

// render executa um template registrando falhas no log.
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	if err := s.Tpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("Failed to render %s: %v", name, err)
	}
}

// logErr registra erros que a UI não tem como exibir, em vez de engoli-los.
func logErr(action string, err error) {
	if err != nil {
		log.Printf("UI: %s: %v", action, err)
	}
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	var userName string
	if session := auth.SessionFromContext(r.Context()); session != nil {
		userName = session.DisplayName
	}
	mediaUsers, err := s.Repo.ListMediaUsers()
	logErr("list media users", err)
	s.render(w, "base.html", map[string]any{
		"Version":        s.Version,
		"MigrationError": s.MigrationError,
		"UserName":       userName,
		"AuthEnabled":    s.Auth != nil,
		"MediaUsers":     mediaUsers,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	attempts, err := s.Repo.ListUnauthorizedAttempts()
	logErr("list unauthorized attempts", err)
	s.render(w, "status.html", map[string]any{
		"Connected":    s.WA.WAClient.IsConnected() && s.WA.WAClient.IsLoggedIn(),
		"HasQR":        s.WA.GetQR() != "",
		"Time":         time.Now().Unix(),
		"Attempts":     attempts,
		"PendingCount": len(attempts),
	})
}

func (s *Server) handleWhatsAppReset(w http.ResponseWriter, r *http.Request) {
	logErr("reset whatsapp session", s.WA.ResetSession())
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
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
	w.Write(png)
}

func (s *Server) handlePhones(w http.ResponseWriter, r *http.Request) {
	phones, err := s.Repo.ListAllowedPhones()
	logErr("list allowed phones", err)
	s.render(w, "phones_list.html", phones)
}

func (s *Server) handleAddPhone(w http.ResponseWriter, r *http.Request) {
	// O WhatsApp identifica o remetente só por dígitos: "+55 (11) 9..." nunca casaria.
	if phone := nonDigits.ReplaceAllString(r.FormValue("phone"), ""); phone != "" {
		logErr("add phone", s.Repo.AddAllowedPhone(phone, r.FormValue("description")))
	}
	s.handlePhones(w, r)
}

func (s *Server) handleDeletePhone(w http.ResponseWriter, r *http.Request) {
	if id, _ := strconv.Atoi(r.URL.Query().Get("id")); id != 0 {
		files, err := s.Repo.DeleteAllowedPhone(id)
		logErr("delete phone", err)
		for _, f := range files {
			os.Remove(f)
		}
	}
	s.handlePhones(w, r)
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	media, err := s.Repo.ListProcessedMedia(r.URL.Query().Get("phone"))
	logErr("list processed media", err)
	hasPending := false
	for _, m := range media {
		if m.Status == "pending" {
			hasPending = true
			break
		}
	}
	s.render(w, "media_list.html", map[string]any{
		"Items":      media,
		"HasPending": hasPending,
	})
}

func (s *Server) handleDeleteMedia(w http.ResponseWriter, r *http.Request) {
	if id, _ := strconv.Atoi(r.URL.Query().Get("id")); id != 0 {
		filePath, err := s.Repo.DeleteProcessedMedia(id)
		logErr("delete media", err)
		if err == nil && filePath != "" {
			os.Remove(filePath)
		}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleListUnauthorized(w http.ResponseWriter, r *http.Request) {
	attempts, err := s.Repo.ListUnauthorizedAttempts()
	logErr("list unauthorized attempts", err)
	s.render(w, "unauthorized_list.html", attempts)
}

func (s *Server) handleAuthorizeUnauthorized(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	name := r.URL.Query().Get("name")
	if phone != "" {
		logErr("authorize phone", s.Repo.AddAllowedPhone(phone, name))
		logErr("delete unauthorized attempt", s.Repo.DeleteUnauthorizedAttempt(phone))
		s.WA.Notify(phone, "Seu acesso no TekstoBot foi autorizado! 🎉\nPor favor, reenvie o seu áudio ou mensagem para que eu possa processar.")
	}
	s.handleListUnauthorized(w, r)
}

func (s *Server) handleDeleteUnauthorized(w http.ResponseWriter, r *http.Request) {
	if phone := r.URL.Query().Get("phone"); phone != "" {
		logErr("delete unauthorized attempt", s.Repo.DeleteUnauthorizedAttempt(phone))
	}
	s.handleListUnauthorized(w, r)
}
