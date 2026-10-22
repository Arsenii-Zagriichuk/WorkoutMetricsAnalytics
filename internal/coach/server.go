package coach

import (
	coachv1 "WorkoutMetricsAnalytics/gen/levl/coach/v1"
	"WorkoutMetricsAnalytics/gen/levl/coach/v1/coachv1connect"
	"context"
	"database/sql"

	"connectrpc.com/connect"
)

type Server struct {
	coachv1connect.UnimplementedCoachServiceHandler
	DB *sql.DB
}

func (s *Server) IngestEvents(ctx context.Context,
	req *connect.Request[coachv1.IngestEventsRequest]) (*connect.Response[coachv1.IngestEventsResponse], error) {

	return nil, nil
}

func (s *Server) GetPrescriptions(ctx context.Context, req *connect.Request[coachv1.GetPrescriptionsRequest]) (*connect.Response[coachv1.GetPrescriptionsResponse], error) {
	return nil, nil
}

func (s *Server) DeleteUser(ctx context.Context, req *connect.Request[coachv1.DeleteUserRequest]) (*connect.Response[coachv1.DeleteUserResponse], error) {
	return nil, nil
}
