package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"WorkoutMetricsAnalytics/internal/middleware"
	"WorkoutMetricsAnalytics/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("test-secret-that-is-at-least-32-bytes!")

// tokenFor signs a valid token for userID, like the one a real client would send.
func tokenFor(t *testing.T, userID string) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    middleware.TokenIssuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(testSecret)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return s
}

// send runs one request through the router in memory and returns the recorded response.
func send(r http.Handler, method, url, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestUserBCannotReadUserAWorkouts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newRouter(store.NewStorage(), testSecret)
	tokenA := tokenFor(t, "user-a")
	tokenB := tokenFor(t, "user-b")

	workout := `{"workout_id":1,"date":"2026-09-28T10:00:00Z","exercises":[{"name":"bench","sets":[{"reps":5,"working_weight":100}]}]}`
	if rec := send(r, http.MethodPost, "/workouts", tokenA, workout); rec.Code != http.StatusCreated {
		t.Fatalf("A save: status %d, want 201; body %s", rec.Code, rec.Body)
	}

	// Every route that reads workouts must be scoped to the caller.
	reads := []struct{ name, url string }{
		{"pr", "/workouts/pr?exercise=bench"},
		{"volume", "/workouts/volume?exercise=bench"},
	}
	for _, rd := range reads {
		t.Run(rd.name, func(t *testing.T) {
			// Control: A sees their own workout. Without this, a save that silently
			// failed would also give B a 404, and the test would prove nothing.
			if rec := send(r, http.MethodGet, rd.url, tokenA, ""); rec.Code != http.StatusOK {
				t.Fatalf("A read: status %d, want 200; body %s", rec.Code, rec.Body)
			}
			if rec := send(r, http.MethodGet, rd.url, tokenB, ""); rec.Code != http.StatusNotFound {
				t.Fatalf("B read: status %d, want 404; body %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestSaveWorkoutRejectsInvalidWorkouts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newRouter(store.NewStorage(), testSecret)
	token := tokenFor(t, "user-a")

	tests := []struct{ name, body string }{
		{"no date", `{"exercises":[{"name":"bench","sets":[{"reps":5,"working_weight":100}]}]}`},
		{"no exercises", `{"date":"2026-09-28T10:00:00Z"}`},
		{"empty exercises", `{"date":"2026-09-28T10:00:00Z","exercises":[]}`},
		{"exercise without a name", `{"date":"2026-09-28T10:00:00Z","exercises":[{"sets":[{"reps":5,"working_weight":100}]}]}`},
		{"exercise without sets", `{"date":"2026-09-28T10:00:00Z","exercises":[{"name":"bench","sets":[]}]}`},
		{"zero reps", `{"date":"2026-09-28T10:00:00Z","exercises":[{"name":"bench","sets":[{"reps":0,"working_weight":100}]}]}`},
		{"negative weight", `{"date":"2026-09-28T10:00:00Z","exercises":[{"name":"bench","sets":[{"reps":5,"working_weight":-100}]}]}`},
		{"bad set in a later exercise", `{"date":"2026-09-28T10:00:00Z","exercises":[{"name":"bench","sets":[{"reps":5,"working_weight":100}]},{"name":"squat","sets":[{"reps":-1,"working_weight":120}]}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if rec := send(r, http.MethodPost, "/workouts", token, tc.body); rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400; body %s", rec.Code, rec.Body)
			}
		})
	}

	// A 400 must also mean nothing was saved.
	if rec := send(r, http.MethodGet, "/workouts/volume?exercise=bench", token, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("after only invalid saves: status %d, want 404; body %s", rec.Code, rec.Body)
	}
}

func TestGetExercisePRsResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newRouter(store.NewStorage(), testSecret)
	token := tokenFor(t, "user-a")

	// The PRs come from different sets: the single gives the one-rep max and the heaviest
	// weight, the set of 5 the volume. The single counts as is, so the one-rep max is 100,
	// not Epley's 103.3, and 5 × 80 only estimates 93.3.
	workout := `{"workout_id":1,"date":"2026-09-28T10:00:00Z","exercises":[{"name":"bench","sets":[{"reps":1,"working_weight":100},{"reps":5,"working_weight":80}]}]}`
	if rec := send(r, http.MethodPost, "/workouts", token, workout); rec.Code != http.StatusCreated {
		t.Fatalf("save: status %d, want 201; body %s", rec.Code, rec.Body)
	}

	rec := send(r, http.MethodGet, "/workouts/pr?exercise=bench", token, "")
	want := `{"one_rep_max":100,"weight":100,"volume":400}`
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Fatalf("got %d %s, want 200 %s", rec.Code, rec.Body, want)
	}
}

func TestReadsReturn404ForAnExerciseNeverLogged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := newRouter(store.NewStorage(), testSecret)
	token := tokenFor(t, "user-a")

	workout := `{"workout_id":1,"date":"2026-09-28T10:00:00Z","exercises":[{"name":"bench","sets":[{"reps":5,"working_weight":100}]}]}`
	if rec := send(r, http.MethodPost, "/workouts", token, workout); rec.Code != http.StatusCreated {
		t.Fatalf("save: status %d, want 201; body %s", rec.Code, rec.Body)
	}

	for _, url := range []string{"/workouts/pr?exercise=squat", "/workouts/volume?exercise=squat"} {
		if rec := send(r, http.MethodGet, url, token, ""); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status %d, want 404; body %s", url, rec.Code, rec.Body)
		}
	}
}
