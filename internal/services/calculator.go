package services

import (
	"WorkoutMetricsAnalytics/internal/models"
	"fmt"
)

// a function which calculates weekly volume of a given exercise;
// and returns a map where year and week is the key(xxxx-Wxx), and volume is the value
func CalculateWeeklyVolume(exercise string, workouts []models.Workout) map[string]float64 {
	volumeTracker := make(map[string]float64)

	// iterate through each workout and get the dates.
	for _, workout := range workouts {
		year, week := workout.Date.ISOWeek()
		date := fmt.Sprintf("%v-W%02d", year, week)

		// iterate through the exercises to get the volume.
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
