package handlers

import (
	"WorkoutMetricsAnalytics/internal/models"
	"WorkoutMetricsAnalytics/internal/store"
	"net/http"

	"github.com/gin-gonic/gin"
)

type WorkoutHandler struct {
	Store *store.Storage
}

func (h *WorkoutHandler) SaveWorkout(c *gin.Context) {
	receivedUserID := c.Param("id")
	if receivedUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user id is required"})
		return
	}

	userID := store.UserID(receivedUserID)

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
