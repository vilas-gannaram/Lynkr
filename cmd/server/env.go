package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Env struct {
	DatabaseURL string
	Port        string
}

func (env *Env) LoadEnv() {
	godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Panic("DATABASE_URL is required, pass it in `.env`")
		return
	}

	port := os.Getenv("PORT")
	if port == "" {
		log.Panic("PORT is require, please it in the `.env`")
		return
	}

	env.DatabaseURL = dbURL
	env.Port = port
}
