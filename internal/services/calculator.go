package services

import (
	"WorkoutMetricsAnalytics/internal/models"
	"fmt"
)

// CalculateWeeklyVolume a function which calculates weekly volume of a given exercise;
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

// CalculateExercisePRs a function to return every type of PR of the given exercise
func CalculateExercisePRs(exercise string, workouts []models.Workout) *models.PR {
	var PrVolume float64
	var PrOneRepMax float64
	var PrWeight float64
	for _, workout := range workouts {
		for _, e := range workout.Exercises {
			if e.Name == exercise {
				for _, set := range e.Sets {
					volume := float64(set.Reps) * set.WorkingWeight
					oneRepMax := set.WorkingWeight * (1.0 + float64(set.Reps)/30.0)
					weight := set.WorkingWeight

					PrVolume = max(PrVolume, volume)
					PrOneRepMax = max(PrOneRepMax, oneRepMax)
					PrWeight = max(PrWeight, weight)
				}
			}
		}
	}
	return &models.PR{
		OneRepMax: PrOneRepMax,
		Weight:    PrWeight,
		Volume:    PrVolume,
	}
}
