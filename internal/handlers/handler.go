package handlers

import (
	"WorkoutMetricsAnalytics/internal/middleware"
	"WorkoutMetricsAnalytics/internal/models"
	"WorkoutMetricsAnalytics/internal/services"
	"WorkoutMetricsAnalytics/internal/store"
	"net/http"

	"github.com/gin-gonic/gin"
)

type WorkoutHandler struct {
	Store *store.Storage
}

// SaveWorkout is handler function for the POST request.
// The main goal of this method is to bind the data received from the client with the workout object.
func (h *WorkoutHandler) SaveWorkout(c *gin.Context) {
	userID := store.UserID(middleware.UserID(c))

	var workout models.Workout

	err := c.ShouldBindBodyWithJSON(&workout)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.Store.SaveWorkout(userID, workout)

	c.JSON(http.StatusCreated, gin.H{
		"message":    "workout saved successfully",
		"session_id": workout.WorkoutID,
	})
}

// GetVolume - a handler for the get request to get the volume from the calculator method
func (h *WorkoutHandler) GetVolume(c *gin.Context) {
	userID := store.UserID(middleware.UserID(c))

	exercise := c.Query("exercise")
	if exercise == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "exercise is required"})
		return
	}

	workouts := h.Store.GetWorkouts(userID)
	if len(workouts) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no workouts found for user"})
		return
	}

	volumeStats := services.CalculateWeeklyVolume(exercise, workouts)

	c.JSON(http.StatusOK, volumeStats)

}

func (h *WorkoutHandler) GetExercisePrs(c *gin.Context) {
	userID := store.UserID(middleware.UserID(c))

	exercise := c.Query("exercise")
	if exercise == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "exercise is required"})
		return
	}

	workouts := h.Store.GetWorkouts(userID)
	if len(workouts) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no workouts found for user"})
		return
	}

	PrStats := services.CalculateExercisePRs(exercise, workouts)

	c.JSON(http.StatusOK, PrStats)
}
