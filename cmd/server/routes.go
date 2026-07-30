package main

import (
	"io/fs"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/vilas-gannaram/Lynkr/ui"
)

type Routes struct {
	Handlers *Handlers
}

func (rt *Routes) Setup() http.Handler {
	mux := chi.NewRouter()

	// Basic CORS
	// for more ideas, see: https://developer.github.com/v3/#cross-origin-resource-sharing
	mux.Use(cors.Handler(cors.Options{
		// AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins: []string{"https://*", "http://*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	mux.Use(middleware.Heartbeat("/ping"))

	staticFS, err := fs.Sub(ui.Files, "static")
	if err != nil {
		log.Fatalf("Could not load static assets: %v", err)
	}
	mux.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.Get("/", rt.Handlers.HomePage)
	mux.Post("/shorten", rt.Handlers.ShortenURL)
	mux.Get("/{shortcode}", rt.Handlers.Redirect)
	mux.Get("/urls", rt.Handlers.ListURLs)

	return mux
}
