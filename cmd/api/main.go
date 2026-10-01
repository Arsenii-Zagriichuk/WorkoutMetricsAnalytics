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
	secret := []byte(os.Getenv("JWT_SECRET"))
	if len(secret) < 32 {
		log.Fatal("JWT_SECRET must be set and at least 32 bytes")
	}

	r := newRouter(store.NewStorage(), secret)

	err2 := r.Run(":8080")
	if err2 != nil {
		return
	}
}

// newRouter registers the middleware and routes. main and the tests both use it,
// so the tests cover exactly the routes the server serves.
func newRouter(db *store.Storage, secret []byte) *gin.Engine {
	r := gin.Default()
	workoutHandler := &handlers.WorkoutHandler{Store: db}

	api := r.Group("/", middleware.RequireAuth(secret))
	api.POST("/workouts", workoutHandler.SaveWorkout)
	api.GET("/workouts/volume", workoutHandler.GetVolume)
	api.GET("/workouts/pr", workoutHandler.GetExercisePrs)
	return r
}
