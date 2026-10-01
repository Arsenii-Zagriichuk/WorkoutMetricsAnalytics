package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	userIDKey = "userID"
)

// RequireAuth verifies the Bearer token and stores the caller's user id in the context.
func RequireAuth(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			unauthorized(c)
			return
		}
		userID, err := VerifyToken(token, secret)
		if err != nil {
			slog.Info("rejected bearer token", "err", err)
			unauthorized(c)
			return
		}
		c.Set(userIDKey, userID)
		c.Next()
	}
}

// UserID returns the authenticated caller's id. Only valid behind RequireAuth.
func UserID(c *gin.Context) string {
	return c.GetString(userIDKey)
}

func unauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", "Bearer")
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing bearer token"})
}

// VerifyToken returns the user id (the "sub" claim) if the token is valid.
func VerifyToken(tokenString string, secret []byte) (string, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
		jwt.WithIssuer("workout-metrics-analytics"),
	)
	if err != nil {
		return "", err
	}
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return "", errors.New("token has no subject")
	}
	return claims.Subject, nil
}
