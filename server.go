package main

import (
	"net/http"
	"text/template"

	_ "embed"
)

type Server struct {
	app *App
}

//go:embed index.gohtml
var indexTpl string

func NewServer(app *App) *Server {
	return &Server{app: app}
}

func (s *Server) Setup() {

	tmpl := template.Must(template.New("index").Parse(indexTpl))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := tmpl.Execute(w, s.app.GetErrors())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
	http.HandleFunc("/refresh", func(rw http.ResponseWriter, r *http.Request) {
		s.app.Load()
		http.Redirect(rw, r, "/", http.StatusTemporaryRedirect)
	})
}

func (s Server) ListenAndServe() error {
	if s.app.config.tlsCert != "" && s.app.config.tlsKey != "" {
		return http.ListenAndServeTLS(
			s.app.config.listenAddr,
			s.app.config.tlsCert,
			s.app.config.tlsKey,
			nil,
		)
	}
	return http.ListenAndServe(s.app.config.listenAddr, nil)
}
