package main

import (
	"WorkoutMetricsAnalytics/internal/handlers"
	"WorkoutMetricsAnalytics/internal/store"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	db := store.NewStorage()
	workoutHandler := &handlers.WorkoutHandler{Store: db}
	r.POST("/users/:id/workouts", workoutHandler.SaveWorkout)
	err := r.Run(":8080")
	if err != nil {
		return
	}
}
