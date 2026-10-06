package main

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type App struct {
	mu    sync.Mutex
	world *world
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	app := &App{world: newWorld()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", app.health)
	mux.HandleFunc("POST /_test/reset", app.reset)
	mux.HandleFunc("GET /_test/export", app.export)
	mux.HandleFunc("POST /_test/import", app.importState)
	mux.HandleFunc("POST /auth/signup", app.signup)
	mux.HandleFunc("POST /auth/login", app.login)
	mux.HandleFunc("GET /me", app.me)
	mux.HandleFunc("POST /payments", app.createPayment)
	mux.HandleFunc("POST /requests", app.createRequest)
	mux.HandleFunc("GET /requests", app.listRequests)
	mux.HandleFunc("POST /requests/{id}/pay", app.payRequest)
	mux.HandleFunc("POST /requests/{id}/decline", app.declineRequest)
	mux.HandleFunc("POST /requests/{id}/cancel", app.cancelRequest)
	mux.HandleFunc("POST /splits", app.createSplit)
	mux.HandleFunc("GET /activity", app.activity)
	mux.HandleFunc("POST /settlements", app.createSettlement)

	srv := &http.Server{
		Addr:              "0.0.0.0:" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
