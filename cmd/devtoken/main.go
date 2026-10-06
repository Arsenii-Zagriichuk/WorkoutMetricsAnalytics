// Command devtoken prints a signed JWT for trying the API locally:
//
//	TOKEN=$(go run ./cmd/devtoken -sub user-a)
//
// It signs with JWT_SECRET (from the environment or .env), the secret the API verifies with.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"WorkoutMetricsAnalytics/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

func main() {
	sub := flag.String("sub", "user-a", "user id to put in the token's sub claim")
	ttl := flag.Duration("ttl", time.Hour, "how long the token stays valid")
	flag.Parse()

	// A missing .env is fine: JWT_SECRET may come from the environment instead.
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("loading .env: %v", err)
	}
	secret := []byte(os.Getenv("JWT_SECRET"))
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET must be set and at least 32 bytes")
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   *sub,
		Issuer:    middleware.TokenIssuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(*ttl)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		log.Fatalf("signing token: %v", err)
	}
	fmt.Println(token)
}
