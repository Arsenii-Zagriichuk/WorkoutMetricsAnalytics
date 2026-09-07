package services

import (
	"WorkoutMetricsAnalytics/internal/models"
	"fmt"
)

// a function which calculates weekly volume of a given exercise;
// and returns a map where year and week is the key, and volume is the value
func calculateWeeklyVolume(exercise string, workouts []models.Workout) map[string]float64 {
	volumeTracker := make(map[string]float64)

	for _, workout := range workouts {
		year, week := workout.Date.ISOWeek()
		date := fmt.Sprintf("%v-W%02d", year, week)

		for _, e := range workout.Exercises {
			if e.Name == exercise {
				for _, set := range e.Sets {
					volume := float64(set.Reps) * set.WorkingWeight

					volumeTracker[date] += volume
				}
			}
		}
	}
	return volumeTracker
}
