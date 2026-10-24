package coach

import (
	coachv1 "WorkoutMetricsAnalytics/gen/levl/coach/v1"
	"testing"

	"google.golang.org/protobuf/proto"
)

func workoutDeletedEvent(workoutID string) *coachv1.Event {
	return event(func(ev *coachv1.Event) {
		ev.Payload = &coachv1.Event_WorkoutDeleted{WorkoutDeleted: &coachv1.WorkoutDeleted{WorkoutId: workoutID}}
	})
}

func bodyWeightRemovedEvent(date string) *coachv1.Event {
	return event(func(ev *coachv1.Event) {
		ev.Payload = &coachv1.Event_BodyWeightRemoved{BodyWeightRemoved: &coachv1.BodyWeightRemoved{Date: date}}
	})
}

func userDeletedEvent() *coachv1.Event {
	return event(func(ev *coachv1.Event) {
		ev.Payload = &coachv1.Event_UserDeleted{UserDeleted: &coachv1.UserDeleted{}}
	})
}

func TestRouteTriageRoutesEachPayloadType(t *testing.T) {
	workout := event(nil)
	routine := routineEvent(&coachv1.RoutineExercise{ExerciseId: "bench", TargetSets: 3})
	profile := profileEvent(nil)
	bodyWeight := bodyWeightEvent("2026-10-22", 82.5)
	nutrition := nutritionEvent(&coachv1.NutritionDayUpdated{Date: "2026-10-23", Calories: 2400})
	userDeleted := userDeletedEvent()

	cases := []struct {
		name string
		ev   *coachv1.Event
		want route
	}{
		{"workout upserted", workout, route{
			eventKind: "workout_upserted", snapshotKind: "workout", aggregateID: "clxaworkout1",
			entity: workout.GetWorkoutUpserted().GetWorkout(),
		}},
		{"workout deleted", workoutDeletedEvent("clxaworkout1"), route{
			eventKind: "workout_deleted", snapshotKind: "workout", aggregateID: "clxaworkout1",
		}},
		{"routine upserted", routine, route{
			eventKind: "routine_upserted", snapshotKind: "routine", aggregateID: "clxaroutine1",
			entity: routine.GetRoutineUpserted().GetRoutine(),
		}},
		{"profile updated", profile, route{
			eventKind: "profile_updated", snapshotKind: "profile", aggregateID: profile.GetUserId(),
			entity: profile.GetProfileUpdated().GetProfile(),
		}},
		{"body weight recorded", bodyWeight, route{
			eventKind: "body_weight_recorded", snapshotKind: "body_weight", aggregateID: "2026-10-22",
			entity: bodyWeight.GetBodyWeightRecorded(),
		}},
		{"body weight removed", bodyWeightRemovedEvent("2026-10-22"), route{
			eventKind: "body_weight_removed", snapshotKind: "body_weight", aggregateID: "2026-10-22",
		}},
		{"nutrition day updated", nutrition, route{
			eventKind: "nutrition_day_updated", snapshotKind: "nutrition_day", aggregateID: "2026-10-23",
			entity: nutrition.GetNutritionDayUpdated(),
		}},
		{"user deleted", userDeleted, route{
			eventKind: "user_deleted", snapshotKind: "account", aggregateID: userDeleted.GetUserId(),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := routeTriage(c.ev)
			if err != nil {
				t.Fatalf("routeTriage: %v", err)
			}
			if got.eventKind != c.want.eventKind {
				t.Errorf("eventKind = %q, want %q", got.eventKind, c.want.eventKind)
			}
			if got.snapshotKind != c.want.snapshotKind {
				t.Errorf("snapshotKind = %q, want %q", got.snapshotKind, c.want.snapshotKind)
			}
			if got.aggregateID != c.want.aggregateID {
				t.Errorf("aggregateID = %q, want %q", got.aggregateID, c.want.aggregateID)
			}
			// Ingest reads a nil entity as a delete, so a delete must give exactly nil
			// and an upsert must give the inner message, not its wrapper.
			if c.want.entity == nil {
				if got.entity != nil {
					t.Errorf("entity is a %T, want a plain nil for a delete", got.entity)
				}
			} else if !proto.Equal(got.entity, c.want.entity) {
				t.Errorf("entity = %v, want %v", got.entity, c.want.entity)
			}
		})
	}
}

// A tombstone only beats an older upsert if both land on the same snapshot row.
func TestRouteTriageUpsertAndDeleteShareASnapshotKey(t *testing.T) {
	pairs := []struct {
		name           string
		upsert, delete *coachv1.Event
	}{
		{"workout", event(nil), workoutDeletedEvent("clxaworkout1")},
		{"body weight", bodyWeightEvent("2026-10-22", 82.5), bodyWeightRemovedEvent("2026-10-22")},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			up, err := routeTriage(p.upsert)
			if err != nil {
				t.Fatalf("routeTriage(upsert): %v", err)
			}
			del, err := routeTriage(p.delete)
			if err != nil {
				t.Fatalf("routeTriage(delete): %v", err)
			}
			if up.snapshotKind != del.snapshotKind || up.aggregateID != del.aggregateID {
				t.Errorf("upsert goes to (%q, %q) but delete to (%q, %q)",
					up.snapshotKind, up.aggregateID, del.snapshotKind, del.aggregateID)
			}
		})
	}
}

// Walks the schema rather than a hand-written list, so adding an event type to the
// proto fails this test until routeTriage handles it, and events.kind always matches
// the proto field name.
func TestRouteTriageCoversEveryPayloadType(t *testing.T) {
	payload := (&coachv1.Event{}).ProtoReflect().Descriptor().Oneofs().ByName("payload")
	for i := 0; i < payload.Fields().Len(); i++ {
		fd := payload.Fields().Get(i)
		t.Run(string(fd.Name()), func(t *testing.T) {
			ev := event(nil)
			m := ev.ProtoReflect()
			m.Set(fd, m.NewField(fd))

			got, err := routeTriage(ev)
			if err != nil {
				t.Fatalf("no route for %s: %v", fd.Name(), err)
			}
			if got.eventKind != string(fd.Name()) {
				t.Errorf("eventKind = %q, want the proto field name %q", got.eventKind, fd.Name())
			}
			if got.snapshotKind == "" {
				t.Error("snapshotKind is empty")
			}
		})
	}
}

func TestRouteTriageFailsWithoutAPayload(t *testing.T) {
	if _, err := routeTriage(event(func(ev *coachv1.Event) { ev.Payload = nil })); err == nil {
		t.Error("want an error for an event without a payload, got nil")
	}
}
