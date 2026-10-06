package models

import "time"

// The binding tags are checked by Gin when a request body is bound, so an invalid
// workout gets a 400 and never reaches the store. dive validates each slice element.

type Set struct {
	Reps          int     `json:"reps" binding:"gte=1"`
	WorkingWeight float64 `json:"working_weight" binding:"gte=0"`
}

type Exercise struct {
	Name string `json:"name" binding:"required"`
	Sets []Set  `json:"sets" binding:"required,min=1,dive"`
}

type Workout struct {
	WorkoutID int        `json:"workout_id"`
	Date      time.Time  `json:"date" binding:"required"`
	Exercises []Exercise `json:"exercises" binding:"required,min=1,dive"`
}

type PR struct {
	OneRepMax float64 `json:"one_rep_max"`
	Weight    float64 `json:"weight"`
	Volume    float64 `json:"volume"`
}
