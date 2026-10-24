package coach

import (
	coachv1 "WorkoutMetricsAnalytics/gen/levl/coach/v1"
	"errors"

	"google.golang.org/protobuf/proto"
)

type route struct {
	eventKind    string
	snapshotKind string
	aggregateID  string
	entity       proto.Message
}

func routeTriage(ev *coachv1.Event) (route, error) {
	switch p := ev.GetPayload().(type) {
	case *coachv1.Event_WorkoutUpserted:
		w := p.WorkoutUpserted.GetWorkout()
		return route{"workout_upserted", "workout", w.GetWorkoutId(), w}, nil
	case *coachv1.Event_WorkoutDeleted:
		return route{"workout_deleted", "workout", p.WorkoutDeleted.GetWorkoutId(), nil}, nil
	case *coachv1.Event_RoutineUpserted:
		r := p.RoutineUpserted.GetRoutine()
		return route{"routine_upserted", "routine", r.GetRoutineId(), r}, nil
	case *coachv1.Event_ProfileUpdated:
		pr := p.ProfileUpdated.GetProfile()
		return route{"profile_updated", "profile", ev.GetUserId(), pr}, nil
	case *coachv1.Event_BodyWeightRecorded:
		return route{"body_weight_recorded", "body_weight", p.BodyWeightRecorded.GetDate(), p.BodyWeightRecorded}, nil
	case *coachv1.Event_BodyWeightRemoved:
		return route{"body_weight_removed", "body_weight", p.BodyWeightRemoved.GetDate(), nil}, nil
	case *coachv1.Event_NutritionDayUpdated:
		return route{"nutrition_day_updated", "nutrition_day", p.NutritionDayUpdated.GetDate(), p.NutritionDayUpdated}, nil
	case *coachv1.Event_UserDeleted:
		return route{"user_deleted", "account", ev.GetUserId(), nil}, nil
	default:
		return route{}, errors.New("no route for this payload")
	}
}
