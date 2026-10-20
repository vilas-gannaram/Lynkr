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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaevor/go-nanoid"
	"github.com/vilas-gannaram/Lynkr/internal/database"
	"github.com/vilas-gannaram/Lynkr/ui"
)

type Data struct {
	Queries *database.Queries
	Pool    *pgxpool.Pool
}

type Handlers struct {
	conn  *Data
	codex *Codex
	cache *Cache
}

type CreateLinkRequest struct {
	TargetURL string      `json:"target_url"`
	Title     pgtype.Text `json:"title"`
}

// Base32-style alpha-numeric characters (without O, 0, I, 1, L, u).
// Using go-nanoid for random string generation
var canonicNanoid, _ = nanoid.CustomASCII("abcdefghjkmnpqrstvwxyz23456789", 8)

// @Method: GET
// @Route: /
// @Desc: Returns HomePage
func (h *Handlers) HomePage(w http.ResponseWriter, r *http.Request) {
	limit := parseQueryInt(r, "limit", 100)
	page := parseQueryInt(r, "page", 1)

	// Ensure limits and offsets stay within reasonable bounds
	if limit < 1 || limit > 100 {
		limit = 10
	}
	if page < 1 {
		page = 1
	}

	offset := (page - 1) * limit

	params := database.ListURLParams{
		Limit:  limit,
		Offset: offset,
	}

	urls, err := h.conn.Queries.ListURL(r.Context(), params)
	if err != nil {
		log.Println("Error fetching urls:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	funcMap := template.FuncMap{
		"inc": func(i int) int { return i + 1 },
	}

	tmpl, err := template.New("home.gohtml").Funcs(funcMap).ParseFS(
		ui.Files,
		"html/pages/home.gohtml",
		"html/layouts/base.gohtml",
		"html/partials/header.gohtml",
		"html/partials/footer.gohtml",
	)
	if err != nil {
		log.Println("Error parsing template:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, urls); err != nil {
		log.Println("Error rendering template:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// @Method: POST
// @Route: /api/links
// @Desc: Generates and returns a shortened URL alias for a given long URL input
func (h *Handlers) CreateLink(w http.ResponseWriter, r *http.Request) {
	var requestPayload CreateLinkRequest

	if err := h.codex.readJSON(w, r, &requestPayload); err != nil {
		h.codex.errorJSON(w, err)
		return
	}

	// Validate URL
	u, err := url.ParseRequestURI(requestPayload.TargetURL)
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
	var created database.Link
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
		created, err = qtx.CreateURL(ctx, database.CreateURLParams{
			Code:      shortKey,
			TargetUrl: requestPayload.TargetURL,
			Title:     requestPayload.Title,
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

	// Warm the cache so the first redirect doesn't miss. Done in the background
	// so a slow/degraded Redis write doesn't delay the API response.
	go h.cache.SetLink(context.Background(), created.Code, created.ID, created.TargetUrl)

	// Build response...
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	fullURL := fmt.Sprintf("%s://%s/%s", scheme, r.Host, shortKey)
	h.codex.writeJSON(w, http.StatusAccepted, fullURL)
}

// @Method: GET
// @Route: /{code}
// @Desc: Accpets the code & redirect user to actual link
func (h *Handlers) Redirect(w http.ResponseWriter, r *http.Request) {
	// Check for the "Prediction" headers
	purpose := r.Header.Get("Sec-Purpose")
	isFakeRequest := strings.Contains(purpose, "prefetch") || strings.Contains(purpose, "prerender")

	shortKey := chi.URLParam(r, "code")
	ctx := r.Context()

	// Cache-aside: try Redis first, only fall back to the DB on a miss
	linkID, targetURL, hit := h.cache.GetLink(ctx, shortKey)
	if !hit {
		link, err := h.conn.Queries.GetURLByCode(ctx, shortKey)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		linkID, targetURL = link.ID, link.TargetUrl
		go h.cache.SetLink(context.Background(), shortKey, linkID, targetURL)
	}

	// Incrementing the count in background, making the redirect faster
	if !isFakeRequest {
		go func(linkID int64) {
			// Use background context for background task to ensure it completes
			// even if the original request context is cancelled.
			err := h.conn.Queries.RecordClick(context.Background(), linkID)
			if err != nil {
				log.Println("Error updating stats:", err)
			}
		}(linkID)
	}

	// Redirecting to the original URL
	http.Redirect(w, r, targetURL, http.StatusFound)
}

// @Method: GET
// @Route: /api/links
// @Desc: Lists all the shorts & long url mappings
func (h *Handlers) ListLinks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := parseQueryInt(r, "limit", 100)
	page := parseQueryInt(r, "page", 1)

	// Ensure limits and offsets stay within reasonable bounds
	if limit < 1 || limit > 100 {
		limit = 10
	}
	if page < 1 {
		page = 1
	}

	offset := (page - 1) * limit

	params := database.ListURLParams{
		Limit:  limit,
		Offset: offset,
	}

	results, err := h.conn.Queries.ListURL(ctx, params)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// @Method: DELETE
// @Route: /api/links/{code}
// @Desc: Delete a short URL
func (h *Handlers) DeleteLink(w http.ResponseWriter, r *http.Request) {
	shortKey := chi.URLParam(r, "code")
	ctx := r.Context()

	deleted, err := h.conn.Queries.DeleteURL(ctx, shortKey)
	if err != nil {
		log.Println("Error deleting link:", err)
		h.codex.errorJSON(w, errors.New("could not delete link"), http.StatusInternalServerError)
		return
	}
	if deleted == 0 {
		h.codex.errorJSON(w, errors.New("link not found"), http.StatusNotFound)
		return
	}

	// Evict after the DB delete so the redirect path can't keep serving it.
	h.cache.DeleteLink(ctx, shortKey)

	w.WriteHeader(http.StatusNoContent)
}
