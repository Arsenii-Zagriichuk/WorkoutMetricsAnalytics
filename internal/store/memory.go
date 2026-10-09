package store

import (
	"WorkoutMetricsAnalytics/internal/models"
	"context"
	"errors"
	"sync"
)

var ErrDuplicateWorkout = errors.New("duplicate workout")

type UserID string

// Store saves and returns each user's workouts. Every implementation returns
// ErrDuplicateWorkout when a user repeats a workout_id.
type Store interface {
	SaveWorkout(ctx context.Context, userID UserID, w models.Workout) error
	GetWorkouts(ctx context.Context, userID UserID) ([]models.Workout, error)
}

var _ Store = (*Storage)(nil) // compile error if Storage stops matching Store

type Storage struct {
	mu             sync.RWMutex
	workoutHistory map[UserID][]models.Workout
}

// NewStorage - constructor for the storage
func NewStorage() *Storage {
	return &Storage{
		workoutHistory: make(map[UserID][]models.Workout),
	}
}

// SaveWorkout - function to save the workout into the storage
func (s *Storage) SaveWorkout(ctx context.Context, id UserID, workout models.Workout) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range s.workoutHistory[id] {
		if w.WorkoutID == workout.WorkoutID {
			return ErrDuplicateWorkout
		}
	}
	s.workoutHistory[id] = append(s.workoutHistory[id], workout)
	return nil
}

// GetWorkouts - function to get all workouts attached to the userID.
// it returns a copy of the workouts to protect the memory and prevent race condition.
func (s *Storage) GetWorkouts(ctx context.Context, id UserID) ([]models.Workout, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	clone := make([]models.Workout, len(s.workoutHistory[id]))
	copy(clone, s.workoutHistory[id])
	return clone, nil
}
