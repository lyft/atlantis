package http

import (
	"net/http"

	"github.com/runatlantis/atlantis/server/logging"
)

type ServerProxy struct {
	*http.Server
	SSLCertFile string
	SSLKeyFile  string
	Logger      logging.Logger
}

func (p *ServerProxy) ListenAndServe() error {
	if p.SSLCertFile != "" && p.SSLKeyFile != "" {
		return p.ListenAndServeTLS(p.SSLCertFile, p.SSLKeyFile)
	}

	return p.Server.ListenAndServe()
}
