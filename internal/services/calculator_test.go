package services

import (
	"math"
	"testing"
	"time"

	"WorkoutMetricsAnalytics/internal/models"
)

func TestEstimateOneRepMax(t *testing.T) {
	tests := []struct {
		name   string
		weight float64
		reps   int
		want   float64
	}{
		{"a single is already a one-rep max", 100, 1, 100},
		{"five reps", 90, 5, 105}, // 90 × (1 + 5/30)
		{"ten reps", 60, 10, 80},  // 60 × (1 + 10/30)
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Floats: compare with a tolerance, not ==.
			if got := estimateOneRepMax(tc.weight, tc.reps); math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("estimateOneRepMax(%v, %d) = %v, want %v", tc.weight, tc.reps, got, tc.want)
			}
		})
	}
}

func TestBodyweightExerciseCountsAsLogged(t *testing.T) {
	workouts := []models.Workout{{
		Date: time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC),
		Exercises: []models.Exercise{
			{Name: "pull-up", Sets: []models.Set{{Reps: 10, WorkingWeight: 0}}},
		},
	}}

	if _, found := CalculateExercisePRs("squat", workouts); found {
		t.Error("PRs for squat, which was never logged: found = true, want false")
	}
	// All-zero PRs are a real result for a bodyweight exercise, not "not found".
	if pr, found := CalculateExercisePRs("pull-up", workouts); !found || pr != (models.PR{}) {
		t.Errorf("PRs for pull-up = %+v, found = %v; want all zeros, found = true", pr, found)
	}
	// The volume route decides 404 by an empty map, so it must agree with the PR route.
	if volume := CalculateWeeklyVolume("pull-up", workouts); len(volume) == 0 {
		t.Error("weekly volume for pull-up is empty, want one week with 0 volume")
	}
}
