package ui

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/skip2/go-qrcode"

	"tekstobot/internal/db"
	"tekstobot/internal/whatsapp"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Server struct {
	Repo           *db.Repository
	WA             *whatsapp.Client
	Tpl            *template.Template
	Version        string
	MigrationError string
}

func NewServer(repo *db.Repository, wa *whatsapp.Client, version string, migrationError string) *Server {
	tpl := template.Must(template.ParseFS(templatesFS, "templates/*.html"))
	return &Server{
		Repo:           repo,
		WA:             wa,
		Tpl:            tpl,
		Version:        version,
		MigrationError: migrationError,
	}
}

func (s *Server) Start(port string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleDashboard)

	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/qr.png", s.handleQR)

	mux.HandleFunc("/phones", s.handlePhones)
	mux.HandleFunc("/phones/add", s.handleAddPhone)
	mux.HandleFunc("/phones/delete", s.handleDeletePhone)

	mux.HandleFunc("/media", s.handleMedia)
	mux.HandleFunc("/media/delete", s.handleDeleteMedia)

	mux.HandleFunc("/unauthorized", s.handleListUnauthorized)
	mux.HandleFunc("/unauthorized/authorize", s.handleAuthorizeUnauthorized)
	mux.HandleFunc("/unauthorized/delete", s.handleDeleteUnauthorized)

	mux.Handle("/data/media/", http.StripPrefix("/data/media/", http.FileServer(http.Dir("data/media"))))

	log.Printf("Starting UI Server on port %s", port)
	return http.ListenAndServe(":"+port, mux)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.Tpl.ExecuteTemplate(w, "base.html", map[string]interface{}{
		"Version":        s.Version,
		"MigrationError": s.MigrationError,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	connected := s.WA.WAClient.IsLoggedIn() && s.WA.WAClient.IsConnected()
	attempts, _ := s.Repo.ListUnauthorizedAttempts()
	s.Tpl.ExecuteTemplate(w, "status.html", map[string]interface{}{
		"Connected":     connected,
		"HasQR":         s.WA.GetQR() != "",
		"Time":          time.Now().Unix(),
		"PendingCount": len(attempts),
		"Attempts":     attempts,
	})
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
	phones, _ := s.Repo.ListAllowedPhones()
	s.Tpl.ExecuteTemplate(w, "phones_list.html", phones)
}

func (s *Server) handleAddPhone(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		phone := r.FormValue("phone")
		desc := r.FormValue("description")
		s.Repo.AddAllowedPhone(phone, desc)
	}
	s.handlePhones(w, r)
}

func (s *Server) handleDeletePhone(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)
	if id != 0 {
		s.Repo.DeleteAllowedPhone(id)
	}
	s.handlePhones(w, r)
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	media, _ := s.Repo.ListProcessedMedia()
	s.Tpl.ExecuteTemplate(w, "media_list.html", media)
}

func (s *Server) handleDeleteMedia(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)
	if id != 0 {
		filePath, err := s.Repo.DeleteProcessedMedia(id)
		if err == nil && filePath != "" {
			os.Remove(filePath)
		}
	}
	s.handleMedia(w, r)
}

func (s *Server) handleListUnauthorized(w http.ResponseWriter, r *http.Request) {
	attempts, _ := s.Repo.ListUnauthorizedAttempts()
	s.Tpl.ExecuteTemplate(w, "unauthorized_list.html", attempts)
}

func (s *Server) handleAuthorizeUnauthorized(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	name := r.URL.Query().Get("name")
	if phone != "" {
		s.Repo.AddAllowedPhone(phone, name)
		s.Repo.DeleteUnauthorizedAttempt(phone)
	}
	// After authorizing, we want to refresh both lists or just the unauthorized one.
	// Since HTMX usually targets one element, we'll return the updated unauthorized list,
	// and use an OOB swap to update the phones list if needed.
	attempts, _ := s.Repo.ListUnauthorizedAttempts()
	s.Tpl.ExecuteTemplate(w, "unauthorized_list.html", attempts)
}

func (s *Server) handleDeleteUnauthorized(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	if phone != "" {
		s.Repo.DeleteUnauthorizedAttempt(phone)
	}
	attempts, _ := s.Repo.ListUnauthorizedAttempts()
	s.Tpl.ExecuteTemplate(w, "unauthorized_list.html", attempts)
}
