package ui

import (
	"testing"

	"tekstobot/internal/auth"
)

// O ServeMux entra em panic se dois padrões conflitarem; as rotas de OIDC só
// existem com Auth definido, então os dois modos precisam ser registrados.
func TestRoutesRegister(t *testing.T) {
	(&Server{}).routes()
	(&Server{Auth: &auth.Auth{}}).routes()
}
