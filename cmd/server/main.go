package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vilas-gannaram/Lynkr/internal/database"
)

type Config struct {
	Data     *Data
	Handlers *Handlers
	Routes   *Routes
	Codex    *Codex
	Env      *Env
}

func main() {
	env := &Env{}
	codex := &Codex{}

	env.LoadEnv()

	pool, err := pgxpool.New(context.Background(), env.DatabaseURL)
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	err = pool.Ping(ctx)
	if err != nil {
		log.Fatalf("Could not connect to Supabase: %v", err)
	}

	cache := NewCache(env.RedisURL)
	defer cache.Close()

	data := &Data{Queries: database.New(pool), Pool: pool}
	handlers := &Handlers{conn: data, codex: codex, cache: cache}
	routes := &Routes{Handlers: handlers}

	app := Config{Data: data, Handlers: handlers, Routes: routes, Codex: codex, Env: env}

	// server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", env.Port),
		Handler: app.Routes.Setup(),
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Panic(err)
	}
}
