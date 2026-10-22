package main

import (
	"WorkoutMetricsAnalytics/gen/levl/coach/v1/coachv1connect"
	"WorkoutMetricsAnalytics/internal/coach"
	"WorkoutMetricsAnalytics/internal/store"
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver for sql.Open
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("loading .env: %v", err)
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	db, err := openDB(databaseURL)
	if err != nil {
		log.Fatalf("connecting to the database: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		log.Fatalf("running migrations: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(coachv1connect.NewCoachServiceHandler(&coach.Server{DB: db}))

	p := new(http.Protocols)
	p.SetHTTP1(true)
	p.SetUnencryptedHTTP2(true)

	srv := &http.Server{Addr: ":9000", Handler: mux, Protocols: p}
	log.Fatal(srv.ListenAndServe())
}

// openDB pings as well as opening: sql.Open doesn't connect, so without the ping a
// wrong DATABASE_URL would only show up on the first request.
func openDB(url string) (*sql.DB, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
