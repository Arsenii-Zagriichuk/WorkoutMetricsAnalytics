package coach

import (
	coachv1 "WorkoutMetricsAnalytics/gen/levl/coach/v1"
	"math"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ptr[T any](v T) *T { return &v }

// event returns a valid WorkoutUpserted event, changed by mutate.
func event(mutate func(ev *coachv1.Event)) *coachv1.Event {
	ev := &coachv1.Event{
		EventId:    "0b6f2c1e-1111-4a2b-9c3d-000000000001",
		UserId:     "clx9k2abcdefghijklmnopqrs",
		Sequence:   1,
		OccurredAt: timestamppb.Now(),
		Payload:    &coachv1.Event_WorkoutUpserted{WorkoutUpserted: &coachv1.WorkoutUpserted{Workout: validWorkout()}},
	}
	if mutate != nil {
		mutate(ev)
	}
	return ev
}

func validWorkout() *coachv1.Workout {
	completed := time.Date(2026, 10, 22, 19, 0, 0, 0, time.UTC)
	return &coachv1.Workout{
		WorkoutId:   "clxaworkout1",
		StartedAt:   timestamppb.New(completed.Add(-time.Hour)),
		CompletedAt: timestamppb.New(completed),
		Source:      coachv1.WorkoutSource_NATIVE,
		Planned:     []*coachv1.PlannedExercise{{ExerciseId: "bench", Order: 0, TargetSets: 3}},
		Exercises: []*coachv1.PerformedExercise{{
			ExerciseId:   "bench",
			Order:        0,
			PlannedOrder: ptr[int32](0),
			Sets: []*coachv1.WorkoutSet{
				{SetNumber: 1, Type: coachv1.SetType_NORMAL, WeightKg: ptr(100.0), Reps: ptr[int32](5), Rpe: ptr(8.0)},
				{SetNumber: 2, Type: coachv1.SetType_WARMUP},
			},
		}},
	}
}

func workoutEvent(mutate func(w *coachv1.Workout)) *coachv1.Event {
	return event(func(ev *coachv1.Event) { mutate(ev.GetWorkoutUpserted().GetWorkout()) })
}

func setEvent(mutate func(s *coachv1.WorkoutSet)) *coachv1.Event {
	return workoutEvent(func(w *coachv1.Workout) { mutate(w.GetExercises()[0].GetSets()[0]) })
}

func routineEvent(exercise *coachv1.RoutineExercise) *coachv1.Event {
	routine := &coachv1.Routine{RoutineId: "clxaroutine1"}
	if exercise != nil {
		routine.Exercises = []*coachv1.RoutineExercise{exercise}
	}
	return event(func(ev *coachv1.Event) {
		ev.Payload = &coachv1.Event_RoutineUpserted{RoutineUpserted: &coachv1.RoutineUpserted{Routine: routine}}
	})
}

func profileEvent(mutate func(p *coachv1.Profile)) *coachv1.Event {
	profile := &coachv1.Profile{Timezone: "Europe/Dublin", WeightUnit: coachv1.WeightUnit_KG}
	if mutate != nil {
		mutate(profile)
	}
	return event(func(ev *coachv1.Event) {
		ev.Payload = &coachv1.Event_ProfileUpdated{ProfileUpdated: &coachv1.ProfileUpdated{Profile: profile}}
	})
}

func bodyWeightEvent(date string, weightKg float64) *coachv1.Event {
	return event(func(ev *coachv1.Event) {
		ev.Payload = &coachv1.Event_BodyWeightRecorded{BodyWeightRecorded: &coachv1.BodyWeightRecorded{Date: date, WeightKg: weightKg}}
	})
}

func nutritionEvent(day *coachv1.NutritionDayUpdated) *coachv1.Event {
	return event(func(ev *coachv1.Event) {
		ev.Payload = &coachv1.Event_NutritionDayUpdated{NutritionDayUpdated: day}
	})
}

// eventFromNewerLevl is an event whose payload uses a oneof field (18) this schema
// doesn't have, decoded from the wire the way the server would decode it.
func eventFromNewerLevl(t *testing.T) *coachv1.Event {
	t.Helper()
	b, err := proto.Marshal(event(func(ev *coachv1.Event) { ev.Payload = nil }))
	if err != nil {
		t.Fatal(err)
	}
	b = protowire.AppendTag(b, 18, protowire.BytesType)
	b = protowire.AppendBytes(b, nil)
	ev := &coachv1.Event{}
	if err := proto.Unmarshal(b, ev); err != nil {
		t.Fatal(err)
	}
	return ev
}

func TestValidateEventAcceptsValidEvents(t *testing.T) {
	cases := []struct {
		name string
		ev   *coachv1.Event
	}{
		{"workout", event(nil)},
		{"uppercase event_id", event(func(ev *coachv1.Event) { ev.EventId = strings.ToUpper(ev.EventId) })},
		{"imported workout", workoutEvent(func(w *coachv1.Workout) { w.Source = coachv1.WorkoutSource_IMPORTED })},
		{"workout with nothing performed", workoutEvent(func(w *coachv1.Workout) { w.Planned, w.Exercises = nil, nil })},
		{"workout deleted", event(func(ev *coachv1.Event) {
			ev.Payload = &coachv1.Event_WorkoutDeleted{WorkoutDeleted: &coachv1.WorkoutDeleted{WorkoutId: "clxaworkout1"}}
		})},
		{"routine without exercises", routineEvent(nil)},
		{"routine without rep target", routineEvent(&coachv1.RoutineExercise{ExerciseId: "bench", TargetSets: 3})},
		{"routine with exact reps", routineEvent(&coachv1.RoutineExercise{
			ExerciseId: "bench", TargetSets: 3, RepTarget: &coachv1.RoutineExercise_ExactReps{ExactReps: 5},
		})},
		{"routine with rep range", routineEvent(&coachv1.RoutineExercise{
			ExerciseId: "bench", TargetSets: 3, RepTarget: &coachv1.RoutineExercise_RepRange{RepRange: &coachv1.RepRange{Min: 8, Max: 10}},
		})},
		{"routine with min == max", routineEvent(&coachv1.RoutineExercise{
			ExerciseId: "bench", TargetSets: 3, RepTarget: &coachv1.RoutineExercise_RepRange{RepRange: &coachv1.RepRange{Min: 5, Max: 5}},
		})},
		{"profile", profileEvent(nil)},
		{"profile without timezone", profileEvent(func(p *coachv1.Profile) { p.Timezone = "" })},
		{"profile with experience and goal", profileEvent(func(p *coachv1.Profile) {
			p.Experience = coachv1.ExperienceLevel_ADVANCED
			p.Goal = coachv1.FitnessGoal_BUILD_MUSCLE
			p.WeeklyWorkoutTarget = ptr[int32](4)
		})},
		{"body weight recorded", bodyWeightEvent("2026-10-22", 82.5)},
		{"body weight removed", event(func(ev *coachv1.Event) {
			ev.Payload = &coachv1.Event_BodyWeightRemoved{BodyWeightRemoved: &coachv1.BodyWeightRemoved{Date: "2026-10-22"}}
		})},
		{"nutrition day", nutritionEvent(&coachv1.NutritionDayUpdated{Date: "2026-10-22", Calories: 2400, ProteinG: 160})},
		{"nutrition day with nothing logged", nutritionEvent(&coachv1.NutritionDayUpdated{Date: "2026-10-22"})},
		{"user deleted", event(func(ev *coachv1.Event) {
			ev.Payload = &coachv1.Event_UserDeleted{UserDeleted: &coachv1.UserDeleted{}}
		})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := validateEvent(c.ev); err != nil {
				t.Errorf("want valid, got %v", err)
			}
		})
	}
}

func TestValidateEventRejectsInvalidEvents(t *testing.T) {
	cases := []struct {
		name    string
		ev      *coachv1.Event
		wantErr string
	}{
		{"event_id not a UUID", event(func(ev *coachv1.Event) { ev.EventId = "evt-1" }), "event_id"},
		{"empty user_id", event(func(ev *coachv1.Event) { ev.UserId = "" }), "user_id"},
		{"sequence 0", event(func(ev *coachv1.Event) { ev.Sequence = 0 }), "sequence"},
		{"negative sequence", event(func(ev *coachv1.Event) { ev.Sequence = -1 }), "sequence"},
		{"no occurred_at", event(func(ev *coachv1.Event) { ev.OccurredAt = nil }), "occurred_at"},
		{"no payload", event(func(ev *coachv1.Event) { ev.Payload = nil }), "payload"},
		{"payload type from a newer Levl", eventFromNewerLevl(t), "payload"},

		{"workout case without a workout", event(func(ev *coachv1.Event) {
			ev.Payload = &coachv1.Event_WorkoutUpserted{WorkoutUpserted: &coachv1.WorkoutUpserted{}}
		}), "workout.workout_id"},
		{"empty workout_id", workoutEvent(func(w *coachv1.Workout) { w.WorkoutId = "" }), "workout.workout_id"},
		{"no started_at", workoutEvent(func(w *coachv1.Workout) { w.StartedAt = nil }), "workout.started_at"},
		{"no completed_at", workoutEvent(func(w *coachv1.Workout) { w.CompletedAt = nil }), "workout.completed_at"},
		{"completed before started", workoutEvent(func(w *coachv1.Workout) {
			w.CompletedAt = timestamppb.New(w.GetStartedAt().AsTime().Add(-time.Minute))
		}), "before started_at"},
		{"unspecified source", workoutEvent(func(w *coachv1.Workout) { w.Source = coachv1.WorkoutSource_WORKOUT_SOURCE_UNSPECIFIED }), "workout.source is unspecified"},
		{"unknown source", workoutEvent(func(w *coachv1.Workout) { w.Source = 9 }), "workout.source has unknown value 9"},
		{"planned exercise without id", workoutEvent(func(w *coachv1.Workout) { w.Planned[0].ExerciseId = "" }), "workout.planned[0].exercise_id"},
		{"negative planned target_sets", workoutEvent(func(w *coachv1.Workout) { w.Planned[0].TargetSets = -1 }), "workout.planned[0].target_sets"},
		{"performed exercise without id", workoutEvent(func(w *coachv1.Workout) { w.Exercises[0].ExerciseId = "" }), "workout.exercises[0].exercise_id"},

		{"unspecified set type", setEvent(func(s *coachv1.WorkoutSet) { s.Type = coachv1.SetType_SET_TYPE_UNSPECIFIED }), "sets[0].type is unspecified"},
		{"unknown set type", setEvent(func(s *coachv1.WorkoutSet) { s.Type = 7 }), "sets[0].type has unknown value 7"},
		{"negative weight", setEvent(func(s *coachv1.WorkoutSet) { s.WeightKg = ptr(-2.5) }), "sets[0].weight_kg"},
		{"NaN weight", setEvent(func(s *coachv1.WorkoutSet) { s.WeightKg = ptr(math.NaN()) }), "sets[0].weight_kg"},
		{"infinite weight", setEvent(func(s *coachv1.WorkoutSet) { s.WeightKg = ptr(math.Inf(1)) }), "sets[0].weight_kg"},
		{"negative reps", setEvent(func(s *coachv1.WorkoutSet) { s.Reps = ptr[int32](-1) }), "sets[0].reps"},
		{"rpe 0", setEvent(func(s *coachv1.WorkoutSet) { s.Rpe = ptr(0.0) }), "sets[0].rpe"},
		{"rpe above 10", setEvent(func(s *coachv1.WorkoutSet) { s.Rpe = ptr(10.5) }), "sets[0].rpe"},
		{"NaN rpe", setEvent(func(s *coachv1.WorkoutSet) { s.Rpe = ptr(math.NaN()) }), "sets[0].rpe"},

		{"workout deleted without id", event(func(ev *coachv1.Event) {
			ev.Payload = &coachv1.Event_WorkoutDeleted{WorkoutDeleted: &coachv1.WorkoutDeleted{}}
		}), "workout_deleted.workout_id"},

		{"empty routine_id", event(func(ev *coachv1.Event) {
			ev.Payload = &coachv1.Event_RoutineUpserted{RoutineUpserted: &coachv1.RoutineUpserted{Routine: &coachv1.Routine{}}}
		}), "routine.routine_id"},
		{"routine exercise without id", routineEvent(&coachv1.RoutineExercise{TargetSets: 3}), "routine.exercises[0].exercise_id"},
		{"negative routine target_sets", routineEvent(&coachv1.RoutineExercise{ExerciseId: "bench", TargetSets: -1}), "routine.exercises[0].target_sets"},
		{"exact reps 0", routineEvent(&coachv1.RoutineExercise{
			ExerciseId: "bench", RepTarget: &coachv1.RoutineExercise_ExactReps{ExactReps: 0},
		}), "exact_reps"},
		{"rep range min 0", routineEvent(&coachv1.RoutineExercise{
			ExerciseId: "bench", RepTarget: &coachv1.RoutineExercise_RepRange{RepRange: &coachv1.RepRange{Min: 0, Max: 8}},
		}), "rep_range"},
		{"rep range max below min", routineEvent(&coachv1.RoutineExercise{
			ExerciseId: "bench", RepTarget: &coachv1.RoutineExercise_RepRange{RepRange: &coachv1.RepRange{Min: 10, Max: 8}},
		}), "rep_range"},
		{"rep range case without a range", routineEvent(&coachv1.RoutineExercise{
			ExerciseId: "bench", RepTarget: &coachv1.RoutineExercise_RepRange{},
		}), "rep_range"},

		{"unknown timezone", profileEvent(func(p *coachv1.Profile) { p.Timezone = "Mars/Olympus" }), "profile.timezone"},
		{"Local timezone", profileEvent(func(p *coachv1.Profile) { p.Timezone = "Local" }), "profile.timezone"},
		{"unspecified weight unit", profileEvent(func(p *coachv1.Profile) { p.WeightUnit = coachv1.WeightUnit_WEIGHT_UNIT_UNSPECIFIED }), "profile.weight_unit is unspecified"},
		{"unknown weight unit", profileEvent(func(p *coachv1.Profile) { p.WeightUnit = 5 }), "profile.weight_unit has unknown value 5"},
		{"unknown experience", profileEvent(func(p *coachv1.Profile) { p.Experience = 9 }), "profile.experience has unknown value 9"},
		{"unknown goal", profileEvent(func(p *coachv1.Profile) { p.Goal = 42 }), "profile.goal has unknown value 42"},
		{"negative weekly target", profileEvent(func(p *coachv1.Profile) { p.WeeklyWorkoutTarget = ptr[int32](-1) }), "profile.weekly_workout_target"},

		{"body weight date that doesn't exist", bodyWeightEvent("2026-02-30", 80), "body_weight_recorded.date"},
		{"body weight date without zero padding", bodyWeightEvent("2026-1-5", 80), "body_weight_recorded.date"},
		{"body weight 0", bodyWeightEvent("2026-10-22", 0), "body_weight_recorded.weight_kg"},
		{"infinite body weight", bodyWeightEvent("2026-10-22", math.Inf(1)), "body_weight_recorded.weight_kg"},
		{"body weight removed with a bad date", event(func(ev *coachv1.Event) {
			ev.Payload = &coachv1.Event_BodyWeightRemoved{BodyWeightRemoved: &coachv1.BodyWeightRemoved{Date: "yesterday"}}
		}), "body_weight_removed.date"},

		{"nutrition day case without a day", nutritionEvent(nil), "nutrition_day_updated.date"},
		{"negative fat", nutritionEvent(&coachv1.NutritionDayUpdated{Date: "2026-10-22", FatG: -1}), "nutrition_day_updated.fat_g"},
		{"NaN calories", nutritionEvent(&coachv1.NutritionDayUpdated{Date: "2026-10-22", Calories: math.NaN()}), "nutrition_day_updated.calories"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateEvent(c.ev)
			if err == nil {
				t.Fatalf("want an error containing %q, got nil", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("want an error containing %q, got %q", c.wantErr, err)
			}
		})
	}
}
