package store

import "WorkoutMetricsAnalytics/internal/models"

type UserID string

type Storage struct {
	WorkoutHistory map[UserID][]models.Workout
}

// NewStorage - constructor for the storage
func NewStorage() *Storage {
	return &Storage{
		WorkoutHistory: make(map[UserID][]models.Workout),
	}
}

// SaveWorkout - function to save the workout into the storage
func (s *Storage) SaveWorkout(id UserID, workout models.Workout) {
	s.WorkoutHistory[id] = append(s.WorkoutHistory[id], workout)
}

// GetWorkouts - function to get all workouts attached to the userID
func (s *Storage) GetWorkouts(id UserID) []models.Workout {
	return s.WorkoutHistory[id]
}
