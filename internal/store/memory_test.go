package store

import (
	"errors"
	"sync"
	"sync/atomic"
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
			// t.Errorf, not t.Fatalf: Fatal may only be called from the test's own goroutine.
			if err := s.SaveWorkout(t.Context(), user, models.Workout{WorkoutID: i + 1, Date: time.Now()}); err != nil {
				t.Errorf("save %d: %v", i+1, err)
			}
		})
		wg.Go(func() {
			_, _ = s.GetWorkouts(t.Context(), user)
		})
	}
	wg.Wait()

	// Two unsynchronised appends can both read the same old slice and one save is lost,
	// so check that every save landed, not just that nothing crashed.
	workouts, err := s.GetWorkouts(t.Context(), user)
	if err != nil {
		t.Fatalf("GetWorkouts: %v", err)
	}
	if len(workouts) != workers {
		t.Fatalf("store has %d workouts after %d saves", len(workouts), workers)
	}
}

// TestStorageRejectsConcurrentDuplicates saves the same workout_id from many goroutines
// at once. Exactly one save may win. If the duplicate check ran outside the lock that
// guards the append, several goroutines could pass the check before any of them
// appended, and the duplicate would be stored. Whether that happens in one attempt is
// down to timing, so the collision is repeated over many rounds, each with a fresh store.
func TestStorageRejectsConcurrentDuplicates(t *testing.T) {
	const rounds, workers = 100, 50
	for round := range rounds {
		saved, duplicates, stored := saveSameWorkoutConcurrently(t, workers)
		if saved != 1 || duplicates != workers-1 || stored != 1 {
			t.Fatalf("round %d: saved %d, rejected %d as duplicates, stored %d; want 1, %d, 1",
				round, saved, duplicates, stored, workers-1)
		}
	}
}

// saveSameWorkoutConcurrently has workers goroutines save one workout_id into a fresh
// store at the same moment, and reports how the saves went and how many were stored.
func saveSameWorkoutConcurrently(t *testing.T, workers int) (saved, duplicates int32, stored int) {
	t.Helper()
	s := NewStorage()
	user := UserID("user-a")
	workout := models.Workout{WorkoutID: 1, Date: time.Now()}

	// Every goroutine waits at start until all have launched, so the saves collide
	// instead of trickling in one after another.
	start := make(chan struct{})
	var savedN, duplicatesN atomic.Int32
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			<-start
			err := s.SaveWorkout(t.Context(), user, workout)
			switch {
			case err == nil:
				savedN.Add(1)
			case errors.Is(err, ErrDuplicateWorkout):
				duplicatesN.Add(1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
	close(start)
	wg.Wait()

	workouts, err := s.GetWorkouts(t.Context(), user)
	if err != nil {
		t.Fatalf("GetWorkouts: %v", err)
	}
	return savedN.Load(), duplicatesN.Load(), len(workouts)
}
