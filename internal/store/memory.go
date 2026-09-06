package store

import "WorkoutMetricsAnalytics/internal/models"

type UserID string

type Storage struct {
	UserHistory map[UserID]models.Workout
}

func NewStorage() *Storage {
	return &Storage{
		UserHistory: make(map[UserID]models.Workout),
	}
}
