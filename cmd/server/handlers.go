package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaevor/go-nanoid"
	"github.com/vilas-gannaram/Lynkr/internal/database"
)

type Data struct {
	Queries *database.Queries
	Pool    *pgxpool.Pool
}

type Handlers struct {
	conn  *Data
	codex *Codex
}

type ShortenRequest struct {
	LongURL string `json:"longURL"`
}

// Base32-style alpha-numeric characters (without O, 0, I, 1, L, u).
// Using go-nanoid for random string generation
var canonicNanoid, _ = nanoid.CustomASCII("abcdefghjkmnpqrstvwxyz23456789", 8)

// @Method: GET
// @Route: /
// @Desc: Returns Home page
func (h *Handlers) HomePage(w http.ResponseWriter, r *http.Request) {
	partials := []string{
		"./cmd/server/templates/partials/header.gohtml",
		"./cmd/server/templates/partials/footer.gohtml",
	}

	layouts := []string{
		"./cmd/server/templates/layouts/base.gohtml",
	}

	page := "./cmd/server/templates/pages/home.gohtml"

	templateSlice := append([]string{page}, layouts...)
	templateSlice = append(templateSlice, partials...)

	tmpl, err := template.ParseFiles(templateSlice...)
	if err != nil {
		log.Println("Error parsing template:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		log.Println("Error rendering template:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// @Method: POST
// @Route: /shorten
// @Desc: Generates and returns a shortened URL alias for a given long URL input
func (h *Handlers) ShortenURL(w http.ResponseWriter, r *http.Request) {
	var requestPayload ShortenRequest

	if err := h.codex.readJSON(w, r, &requestPayload); err != nil {
		h.codex.errorJSON(w, err)
		return
	}

	// Validate URL
	u, err := url.ParseRequestURI(requestPayload.LongURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	// Prevent Self-Shortening (Loop Check)
	if u.Host == r.Host {
		http.Error(w, "Cannot shorten URLs from this domain", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var shortKey string
	var success bool
	var lastErr error

	for i := 0; i < 3; i++ {
		shortKey = canonicNanoid()

		// 1. Start a NEW transaction for each attempt
		tx, err := h.conn.Pool.Begin(ctx)
		if err != nil {
			lastErr = err
			continue
		}

		// 2. Try the insert
		qtx := h.conn.Queries.WithTx(tx)
		_, err = qtx.CreateURL(ctx, database.CreateURLParams{
			ShortCode:   shortKey,
			OriginalUrl: requestPayload.LongURL,
		})

		if err == nil {
			// 3. Commit only on success
			if err = tx.Commit(ctx); err == nil {
				success = true
				break
			}
		}

		// 4. If we reach here, something failed. Rollback this attempt.
		lastErr = err
		tx.Rollback(ctx)
	}

	if !success {
		// Log the actual error to your terminal/Render logs
		fmt.Printf("Final failure after 3 attempts. Last error: %v\n", lastErr)
		http.Error(w, "Could not generate unique short code", http.StatusConflict)
		return
	}

	// Build response...
	fullURL := fmt.Sprintf("https://%s/%s", r.Host, shortKey)
	h.codex.writeJSON(w, http.StatusAccepted, fullURL)
}

// @Method: GET
// @Route: /{shortcode}
// @Desc: Accpets the shortcode & redirect user to actual link
func (h *Handlers) Redirect(w http.ResponseWriter, r *http.Request) {
	// Check for the "Prediction" headers
	purpose := r.Header.Get("Sec-Purpose")
	isFakeRequest := strings.Contains(purpose, "prefetch") || strings.Contains(purpose, "prerender")

	shortKey := chi.URLParam(r, "shortcode")

	// Fetching the mapping from DB
	ctx := r.Context()
	urlMapping, err := h.conn.Queries.GetURLByCode(ctx, shortKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Incrementing the count in background, making the redirect faster
	if !isFakeRequest {
		go func(urlID int64) {
			// Use background context for background task to ensure it completes
			// even if the original request context is cancelled.
			err := h.conn.Queries.UpsertStats(context.Background(), urlID)
			if err != nil {
				log.Println("Error updating stats:", err)
			}
		}(urlMapping.ID)
	}

	// Redirecting to the original URL
	http.Redirect(w, r, urlMapping.OriginalUrl, http.StatusFound)
}

// @Method: GET
// @Route: /urls
// @Desc: Lists all the shorts & long url mappings
func (h *Handlers) ListURLs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	results, err := h.conn.Queries.ListURL(ctx)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
