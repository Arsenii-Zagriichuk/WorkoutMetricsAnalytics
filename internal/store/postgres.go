package store

import (
	"WorkoutMetricsAnalytics/internal/models"
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver for sql.Open
)

type PostgresStore struct {
	db *sql.DB
}

var _ Store = (*PostgresStore)(nil)

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// SaveWorkout - function to save the workout into the storage
func (s *PostgresStore) SaveWorkout(ctx context.Context, userID UserID, w models.Workout) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var workoutID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO workouts (user_id, client_workout_id, performed_at)
		 VALUES ($1, $2, $3) RETURNING id`,
		userID, w.WorkoutID, w.Date,
	).Scan(&workoutID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateWorkout
		}
		return err
	}

	for i, e := range w.Exercises {
		var exerciseID int64
		err := tx.QueryRowContext(ctx,
			`INSERT INTO exercises (workout_id, position, name) VALUES ($1, $2, $3) RETURNING id`,
			workoutID, i, e.Name,
		).Scan(&exerciseID)
		if err != nil {
			return err
		}
		for j, set := range e.Sets {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO sets (exercise_id, position, reps, working_weight) VALUES ($1, $2, $3, $4)`,
				exerciseID, j, set.Reps, set.WorkingWeight,
			)
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// GetWorkouts returns the user's workouts, oldest first, with exercises and sets in the
// order they were saved. The query returns one row per set, and the loop rebuilds the
// workout → exercise → set nesting from those flat rows.
func (s *PostgresStore) GetWorkouts(ctx context.Context, userID UserID) ([]models.Workout, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT w.id, w.client_workout_id, w.performed_at, e.id, e.name, s.reps, s.working_weight
		 FROM workouts w
		 JOIN exercises e ON e.workout_id = w.id
		 JOIN sets s ON s.exercise_id = e.id
		 WHERE w.user_id = $1
		 ORDER BY w.performed_at, w.id, e.position, s.position`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Empty, not nil, so a user with no workouts gets the same result as from the memory store.
	workouts := []models.Workout{}
	// Database ids start at 1, so these zero values never match the first row.
	var lastWorkoutID, lastExerciseID int64
	for rows.Next() {
		var (
			workoutID, exerciseID int64
			clientWorkoutID       int
			performedAt           time.Time
			name                  string
			set                   models.Set
		)
		if err := rows.Scan(&workoutID, &clientWorkoutID, &performedAt, &exerciseID, &name, &set.Reps, &set.WorkingWeight); err != nil {
			return nil, err
		}

		// Rows arrive sorted, so a new id means the previous workout or exercise is complete.
		if workoutID != lastWorkoutID {
			// pgx returns timestamps in the machine's time zone; UTC keeps ISO weeks the same everywhere.
			workouts = append(workouts, models.Workout{WorkoutID: clientWorkoutID, Date: performedAt.UTC()})
			lastWorkoutID = workoutID
		}
		// Taken fresh on every row and never kept across an append, which can move the slice.
		w := &workouts[len(workouts)-1]
		if exerciseID != lastExerciseID {
			w.Exercises = append(w.Exercises, models.Exercise{Name: name})
			lastExerciseID = exerciseID
		}
		e := &w.Exercises[len(w.Exercises)-1]
		e.Sets = append(e.Sets, set)
	}
	// A connection that fails halfway through the rows only shows up here.
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return workouts, nil
}
