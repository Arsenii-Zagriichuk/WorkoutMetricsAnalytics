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

type Session struct {
	SessionID int        `json:"session_id"`
	Date      time.Time  `json:"date"`
	Exercises []Exercise `json:"exercises"`
}
