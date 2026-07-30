package main

import (
	"encoding/json"
	"io"
	"net/http"
)

type jsonResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// @desc: Read/Write JSON at HTTP layer
type Codex struct{}

func (c *Codex) readJSON(w http.ResponseWriter, r *http.Request, data any) error {
	maxBytes := 1048576 // 1MB

	//
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	// Decode the JSON from the body
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(data); err != nil {
		return err
	}

	// Check if there is more than one JSON object in the body
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return err
	}

	return nil
}

func (c *Codex) writeJSON(w http.ResponseWriter, status int, data any, headers ...http.Header) error {
	out, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_, err = w.Write(out)
	if err != nil {
		return err
	}
	return nil
}

func (c *Codex) errorJSON(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest
	if len(status) > 0 {
		statusCode = status[0]
	}

	payload := jsonResponse{
		Error:   true,
		Message: err.Error(),
	}

	return c.writeJSON(w, statusCode, payload)
}
