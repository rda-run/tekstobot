package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"tekstobot/internal/auth"
)

// O ServeMux entra em panic se dois padrões conflitarem; as rotas de OIDC só
// existem com Auth definido, então os dois modos precisam ser registrados.
func TestRoutesRegister(t *testing.T) {
	(&Server{}).routes()
	(&Server{Auth: &auth.Auth{}}).routes()
}

// Sem sessão, a interface (tela de login) abre e a API responde 401.
func TestUnauthenticated(t *testing.T) {
	mux := (&Server{Auth: &auth.Auth{}}).routes()
	for path, want := range map[string]int{
		"/":            http.StatusOK,
		"/api/info":    http.StatusUnauthorized,
		"/api/media":   http.StatusUnauthorized,
		"/qr.png":      http.StatusFound,
		"/data/media/": http.StatusFound,
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != want {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, want)
		}
	}
}
