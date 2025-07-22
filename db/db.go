package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// DB ek global variable hai jo database connection pool ko hold karta hai.
var DB *pgxpool.Pool

func InitDB() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v. Proceeding with environment variables.", err)
	}

	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	if user == "" || password == "" || host == "" || port == "" || dbname == "" {
		log.Fatal("Error: One or more database environment variables are not set (DB_USER, DB_PASSWORD, DB_HOST, DB_PORT, DB_NAME).")
	}

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, dbname)

	ctx := context.Background()
	var err error
	// pgxpool ka use karke naya connection pool banate hain.
	DB, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Error creating DB pool: %v\n", err)
	}

	// Database ko ping karke connection check karte hain.
	if err = DB.Ping(ctx); err != nil {
		log.Fatalf("Error connecting to DB: %v\n", err)
	}

	log.Println("-> Successfully connected to DB")
}