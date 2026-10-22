package coach

import (
	coachv1 "WorkoutMetricsAnalytics/gen/levl/coach/v1"
	"errors"
	"fmt"
	"math"
	"regexp"
	"time"
	_ "time/tzdata" // time.LoadLocation needs a timezone database, and slim container images ship none

	"google.golang.org/protobuf/reflect/protoreflect"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// validateEvent returns why an event can never be applied, or nil. The message becomes
// the REJECTED reason, so it names fields the way the proto spells them.
func validateEvent(ev *coachv1.Event) error {
	if !uuidRegex.MatchString(ev.GetEventId()) {
		return errors.New("event_id must be a UUID")
	}
	if ev.GetUserId() == "" {
		return errors.New("user_id is empty")
	}
	if ev.GetSequence() <= 0 {
		return errors.New("sequence must be greater than 0")
	}
	if err := ev.GetOccurredAt().CheckValid(); err != nil {
		return fmt.Errorf("occurred_at: %w", err)
	}

	switch p := ev.GetPayload().(type) {
	case *coachv1.Event_WorkoutUpserted:
		return validateWorkout(p.WorkoutUpserted.GetWorkout())
	case *coachv1.Event_WorkoutDeleted:
		if p.WorkoutDeleted.GetWorkoutId() == "" {
			return errors.New("workout_deleted.workout_id is empty")
		}
		return nil
	case *coachv1.Event_RoutineUpserted:
		return validateRoutine(p.RoutineUpserted.GetRoutine())
	case *coachv1.Event_ProfileUpdated:
		return validateProfile(p.ProfileUpdated.GetProfile())
	case *coachv1.Event_BodyWeightRecorded:
		if err := validateDate("body_weight_recorded.date", p.BodyWeightRecorded.GetDate()); err != nil {
			return err
		}
		if w := p.BodyWeightRecorded.GetWeightKg(); !isFinite(w) || w <= 0 {
			return errors.New("body_weight_recorded.weight_kg must be greater than 0")
		}
		return nil
	case *coachv1.Event_BodyWeightRemoved:
		return validateDate("body_weight_removed.date", p.BodyWeightRemoved.GetDate())
	case *coachv1.Event_NutritionDayUpdated:
		return validateNutritionDay(p.NutritionDayUpdated)
	case *coachv1.Event_UserDeleted:
		return nil
	default:
		return errors.New("payload is empty or of an unknown type")
	}
}

func validateWorkout(w *coachv1.Workout) error {
	if w.GetWorkoutId() == "" {
		return errors.New("workout.workout_id is empty")
	}
	if err := w.GetStartedAt().CheckValid(); err != nil {
		return fmt.Errorf("workout.started_at: %w", err)
	}
	if err := w.GetCompletedAt().CheckValid(); err != nil {
		return fmt.Errorf("workout.completed_at: %w", err)
	}
	if w.GetCompletedAt().AsTime().Before(w.GetStartedAt().AsTime()) {
		return errors.New("workout.completed_at is before started_at")
	}
	if err := requireEnum("workout.source", w.GetSource()); err != nil {
		return err
	}
	for i, e := range w.GetPlanned() {
		if e.GetExerciseId() == "" {
			return fmt.Errorf("workout.planned[%d].exercise_id is empty", i)
		}
		if e.GetTargetSets() < 0 {
			return fmt.Errorf("workout.planned[%d].target_sets is negative", i)
		}
	}
	for i, e := range w.GetExercises() {
		if e.GetExerciseId() == "" {
			return fmt.Errorf("workout.exercises[%d].exercise_id is empty", i)
		}
		for j, s := range e.GetSets() {
			if err := validateSet(s); err != nil {
				return fmt.Errorf("workout.exercises[%d].sets[%d].%w", i, j, err)
			}
		}
	}
	return nil
}

func validateSet(s *coachv1.WorkoutSet) error {
	if err := requireEnum("type", s.GetType()); err != nil {
		return err
	}
	// An unset optional reads as 0, which passes both of these.
	if w := s.GetWeightKg(); !isFinite(w) || w < 0 {
		return errors.New("weight_kg must be 0 or more")
	}
	if s.GetReps() < 0 {
		return errors.New("reps is negative")
	}
	// Here 0 would be wrong, so presence is checked first.
	if s.Rpe != nil && (!isFinite(s.GetRpe()) || s.GetRpe() <= 0 || s.GetRpe() > 10) {
		return errors.New("rpe must be above 0 and at most 10")
	}
	return nil
}

func validateRoutine(r *coachv1.Routine) error {
	if r.GetRoutineId() == "" {
		return errors.New("routine.routine_id is empty")
	}
	for i, e := range r.GetExercises() {
		if e.GetExerciseId() == "" {
			return fmt.Errorf("routine.exercises[%d].exercise_id is empty", i)
		}
		if e.GetTargetSets() < 0 {
			return fmt.Errorf("routine.exercises[%d].target_sets is negative", i)
		}
		switch t := e.GetRepTarget().(type) {
		case *coachv1.RoutineExercise_ExactReps:
			if t.ExactReps < 1 {
				return fmt.Errorf("routine.exercises[%d].exact_reps must be at least 1", i)
			}
		case *coachv1.RoutineExercise_RepRange:
			if lo, hi := t.RepRange.GetMin(), t.RepRange.GetMax(); lo < 1 || hi < lo {
				return fmt.Errorf("routine.exercises[%d].rep_range needs 1 <= min <= max", i)
			}
		}
	}
	return nil
}

func validateProfile(p *coachv1.Profile) error {
	// Levl has users without an experience level or goal, so unset is fine; an unknown
	// value isn't.
	if err := knownEnum("profile.experience", p.GetExperience()); err != nil {
		return err
	}
	if err := knownEnum("profile.goal", p.GetGoal()); err != nil {
		return err
	}
	if p.GetWeeklyWorkoutTarget() < 0 {
		return errors.New("profile.weekly_workout_target is negative")
	}
	if tz := p.GetTimezone(); tz != "" {
		// LoadLocation("Local") would return this server's own zone.
		if _, err := time.LoadLocation(tz); err != nil || tz == "Local" {
			return fmt.Errorf("profile.timezone %q is not an IANA timezone", tz)
		}
	}
	return requireEnum("profile.weight_unit", p.GetWeightUnit())
}

func validateNutritionDay(n *coachv1.NutritionDayUpdated) error {
	if err := validateDate("nutrition_day_updated.date", n.GetDate()); err != nil {
		return err
	}
	totals := []struct {
		field string
		value float64
	}{
		{"calories", n.GetCalories()},
		{"protein_g", n.GetProteinG()},
		{"carbs_g", n.GetCarbsG()},
		{"fat_g", n.GetFatG()},
	}
	for _, t := range totals {
		if !isFinite(t.value) || t.value < 0 {
			return fmt.Errorf("nutrition_day_updated.%s must be 0 or more", t.field)
		}
	}
	return nil
}

func validateDate(field, date string) error {
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return fmt.Errorf("%s must be a YYYY-MM-DD date", field)
	}
	return nil
}

// requireEnum rejects a value that is unset, or that this engine doesn't know yet
// (proto enums accept any number, so a newer Levl can send one).
func requireEnum(field string, v protoreflect.Enum) error {
	if v.Number() == 0 {
		return fmt.Errorf("%s is unspecified", field)
	}
	return knownEnum(field, v)
}

func knownEnum(field string, v protoreflect.Enum) error {
	if v.Descriptor().Values().ByNumber(v.Number()) == nil {
		return fmt.Errorf("%s has unknown value %d", field, v.Number())
	}
	return nil
}

// isFinite rejects NaN and ±Inf, which pass every < and > comparison check.
func isFinite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}
