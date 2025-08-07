package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var DB *pgxpool.Pool

func InitDB() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v.", err)
	}

	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	if user == "" || password == "" || host == "" || port == "" || dbname == "" {
		log.Fatal("Database environment variables are not set.")
	}

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, dbname)

	ctx := context.Background()
	var err error
	DB, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Error creating DB pool: %v\n", err)
	}

	if err = DB.Ping(ctx); err != nil {
		log.Fatalf("Error connecting to DB: %v\n", err)
	}

	log.Println("Successfully connected to DB")
}