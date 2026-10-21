package openapi

import (
	"embed"
	"net/http"
)

//go:generate go run ../../cmd/openapi -root ../.. -out openapi.json
//go:embed openapi.json static/*
var assets embed.FS

func Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, r *http.Request) {
		data, _ := assets.ReadFile("openapi.json")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/swagger/", 302) })
	mux.HandleFunc("GET /swagger/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[len("/swagger/"):]
		if path == "" {
			path = "index.html"
		}
		switch path {
		case "index.html", "swagger-ui-bundle.js", "swagger-ui.css", "init.js", "LICENSE":
		default:
			http.NotFound(w, r)
			return
		}
		data, err := assets.ReadFile("static/" + path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		switch path {
		case "index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case "swagger-ui.css":
			w.Header().Set("Content-Type", "text/css")
		case "init.js", "swagger-ui-bundle.js":
			w.Header().Set("Content-Type", "application/javascript")
		default:
			w.Header().Set("Content-Type", "text/plain")
		}
		_, _ = w.Write(data)
	})
}
