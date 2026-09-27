package models

import "time"

type Set struct {
	Reps          int     `json:"reps"`
	WorkingWeight float64 `json:"working_weight"`
}

type Exercise struct {
	Name string `json:"name"`
	Sets []Set  `json:"sets"`
}

type Workout struct {
	WorkoutID int        `json:"workout_id"`
	Date      time.Time  `json:"date"`
	Exercises []Exercise `json:"exercises"`
}

type PR struct {
	OneRepMax float64
	Weight    float64
	Volume    float64
}
