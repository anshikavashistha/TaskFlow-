package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	JWTSecret     string
	Port          string
	Seed          bool
}

func Load() (Config, error) {
	_ = godotenv.Load()

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	jwtSecret := os.Getenv("JWT_SECRET")

	if jwtSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	if dbHost == "" || dbPort == "" || dbUser == "" || dbPassword == "" || dbName == "" {
		return Config{}, fmt.Errorf("DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, and DB_NAME are required")
	}

	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName,
	)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	seed := parseBool(os.Getenv("SEED"))

	return Config{
		DatabaseURL: databaseURL,
		JWTSecret:     jwtSecret,
		Port:          port,
		Seed:          seed,
	}, nil
}

func parseBool(s string) bool {
	if s == "" {
		return false
	}
	v, err := strconv.ParseBool(s)
	return err == nil && v
}
