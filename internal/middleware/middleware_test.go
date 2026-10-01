package middleware

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("test-secret-that-is-at-least-32-bytes!")

// signToken creates a signed token so each case can change one thing.
func signToken(t *testing.T, method jwt.SigningMethod, secret []byte, claims jwt.RegisteredClaims) string {
	t.Helper() // on failure, Go reports the line that called signToken, not this one
	s, err := jwt.NewWithClaims(method, claims).SignedString(secret)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return s
}

func TestVerifyToken(t *testing.T) {
	inAnHour := jwt.NewNumericDate(time.Now().Add(time.Hour))

	tests := []struct {
		name    string
		token   string
		wantSub string
		wantErr bool
	}{
		{
			name:    "valid",
			token:   signToken(t, jwt.SigningMethodHS256, testSecret, jwt.RegisteredClaims{Subject: "user-a", Issuer: TokenIssuer, ExpiresAt: inAnHour}),
			wantSub: "user-a",
		},
		{
			name:    "expired",
			token:   signToken(t, jwt.SigningMethodHS256, testSecret, jwt.RegisteredClaims{Subject: "user-a", Issuer: TokenIssuer, ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))}),
			wantErr: true,
		},
		// your turn: wrong secret, HS512, no sub, wrong issuer
		{
			name:    "invalid secret",
			token:   signToken(t, jwt.SigningMethodHS256, make([]byte, 0), jwt.RegisteredClaims{Subject: "user-a", Issuer: TokenIssuer, ExpiresAt: inAnHour}),
			wantErr: true,
		},
		{
			name:    "invalid method",
			token:   signToken(t, jwt.SigningMethodHS512, testSecret, jwt.RegisteredClaims{Subject: "user-a", Issuer: TokenIssuer, ExpiresAt: inAnHour}),
			wantErr: true,
		},
		{
			name:    "no sub",
			token:   signToken(t, jwt.SigningMethodHS256, testSecret, jwt.RegisteredClaims{Subject: "", Issuer: TokenIssuer, ExpiresAt: inAnHour}),
			wantErr: true,
		},
		{
			name:    "invalid issuer",
			token:   signToken(t, jwt.SigningMethodHS256, testSecret, jwt.RegisteredClaims{Subject: "user-a", Issuer: "someone-else", ExpiresAt: inAnHour}),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sub, err := VerifyToken(tc.token, testSecret)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got sub=%q", sub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sub != tc.wantSub {
				t.Errorf("sub = %q, want %q", sub, tc.wantSub)
			}
		})
	}
}
