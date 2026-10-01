package store

import (
	"WorkoutMetricsAnalytics/internal/models"
	"sync"
)

type UserID string

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
func (s *Storage) SaveWorkout(id UserID, workout models.Workout) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workoutHistory[id] = append(s.workoutHistory[id], workout)
}

// GetWorkouts - function to get all workouts attached to the userID.
// it returns a copy of the workouts to protect the memory and prevent race condition.
func (s *Storage) GetWorkouts(id UserID) []models.Workout {
	s.mu.RLock()
	defer s.mu.RUnlock()
	clone := make([]models.Workout, len(s.workoutHistory[id]))
	copy(clone, s.workoutHistory[id])
	return clone
}
