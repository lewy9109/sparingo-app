package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"sqoush-app/internal/store"
	"sqoush-app/internal/web"
)

//go:embed templates/* templates/partials/* static/* static/css/* static/img/*
var content embed.FS

func main() {
	templates, err := web.NewTemplates(content)
	if err != nil {
		log.Fatalf("templates: %v", err)
	}
	store := store.NewMemoryStore()
	server := web.NewServer(store, templates)
	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") == "" {
		_ = godotenv.Load(".env", ".env.local")
	}

	// 1. Inicjalizacja Routera (Chi)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return web.WithCurrentUser(store, next)
	})

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	r.Mount("/", server.Routes())

	// 2. Wykrywanie środowiska (AWS Lambda vs Localhost)
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		// Jesteśmy na AWS -> Uruchom adapter Lambdy
		log.Println("Uruchamianie w trybie Lambda...")
		adapter := httpadapter.New(r)
		lambda.Start(adapter.ProxyWithContext)
	} else {
		// Jesteśmy lokalnie -> Uruchom zwykły serwer HTTP
		log.Println("Uruchamianie lokalnie na :8080...")
		http.ListenAndServe(":8080", r)
	}
}
