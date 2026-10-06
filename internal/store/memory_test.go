package store

import (
	"sync"
	"testing"
	"time"

	"WorkoutMetricsAnalytics/internal/models"
)

// TestStorageConcurrentAccess saves and reads one user's workouts from many goroutines
// at once. Run it with -race: without the mutex in Storage, the race detector reports
// a data race on the workoutHistory map and the test fails.
func TestStorageConcurrentAccess(t *testing.T) {
	const workers = 50
	s := NewStorage()
	user := UserID("user-a")

	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			s.SaveWorkout(user, models.Workout{WorkoutID: i, Date: time.Now()})
		})
		wg.Go(func() {
			_ = s.GetWorkouts(user)
		})
	}
	wg.Wait()

	// Two unsynchronised appends can both read the same old slice and one save is lost,
	// so check that every save landed, not just that nothing crashed.
	if got := len(s.GetWorkouts(user)); got != workers {
		t.Fatalf("store has %d workouts after %d saves", got, workers)
	}
}
