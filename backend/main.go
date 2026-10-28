package main

import (
	"context"
	"log"
	"os"

	"backend/cmd"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// @title		Budgeting App API
// @version	1.0
// @BasePath	/
func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("no .env loaded: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	api := cmd.NewAPI(cmd.APIConfig{Addr: ":8080", DB: pool})
	api.Mount()
	api.Run()
}
