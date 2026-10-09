package handlers

import (
	"WorkoutMetricsAnalytics/internal/middleware"
	"WorkoutMetricsAnalytics/internal/models"
	"WorkoutMetricsAnalytics/internal/services"
	"WorkoutMetricsAnalytics/internal/store"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

type WorkoutHandler struct {
	Store store.Store
}

// SaveWorkout is handler function for the POST request.
// The main goal of this method is to bind the data received from the client with the workout object.
func (h *WorkoutHandler) SaveWorkout(c *gin.Context) {
	userID := store.UserID(middleware.UserID(c))

	var workout models.Workout
	if err := c.ShouldBindBodyWithJSON(&workout); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.Store.SaveWorkout(c.Request.Context(), userID, workout)
	if errors.Is(err, store.ErrDuplicateWorkout) {
		c.JSON(http.StatusConflict, gin.H{"error": "a workout with this workout_id already exists"})
		return
	}

	if err != nil {
		slog.Error("saving workout", "user_id", userID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

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

	workouts, err := h.Store.GetWorkouts(c.Request.Context(), userID)
	if err != nil {
		slog.Error("reading workouts", "user_id", userID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	volumeStats := services.CalculateWeeklyVolume(exercise, workouts)
	if len(volumeStats) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no workouts found for this exercise"})
		return
	}

	c.JSON(http.StatusOK, volumeStats)
}

func (h *WorkoutHandler) GetExercisePrs(c *gin.Context) {
	userID := store.UserID(middleware.UserID(c))

	exercise := c.Query("exercise")
	if exercise == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "exercise is required"})
		return
	}

	workouts, err := h.Store.GetWorkouts(c.Request.Context(), userID)
	if err != nil {
		slog.Error("reading workouts", "user_id", userID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	prStats, found := services.CalculateExercisePRs(exercise, workouts)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "no workouts found for this exercise"})
		return
	}

	c.JSON(http.StatusOK, prStats)
}
