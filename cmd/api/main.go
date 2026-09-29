package main

import (
	"WorkoutMetricsAnalytics/internal/handlers"
	"WorkoutMetricsAnalytics/internal/middleware"
	"WorkoutMetricsAnalytics/internal/store"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err1 := godotenv.Load()
	if err1 != nil {
		return
	}
	r := gin.Default()
	db := store.NewStorage()
	workoutHandler := &handlers.WorkoutHandler{Store: db}
	// main.go
	secret := []byte(os.Getenv("JWT_SECRET"))
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET must be set and at least 32 bytes")
	}
	api := r.Group("/", middleware.RequireAuth(secret))
	api.POST("/workouts", workoutHandler.SaveWorkout)
	api.GET("/workouts/volume", workoutHandler.GetVolume)
	api.GET("/workouts/pr", workoutHandler.GetExercisePrs)

	err2 := r.Run(":8080")
	if err2 != nil {
		return
	}
}
