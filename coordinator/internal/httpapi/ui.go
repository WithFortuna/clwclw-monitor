package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed ui/*
var uiEmbedFS embed.FS

var uiFS fs.FS

func init() {
	sub, err := fs.Sub(uiEmbedFS, "ui")
	if err != nil {
		// If this fails, the binary is built incorrectly; still keep server functional.
		uiFS = nil
		return
	}
	uiFS = sub
}

func (s *Server) registerUI() {
	if uiFS == nil {
		return
	}
	s.mux.Handle("/", http.FileServer(http.FS(uiFS)))
}

