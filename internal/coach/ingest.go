package coach

import (
	coachv1 "WorkoutMetricsAnalytics/gen/levl/coach/v1"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"
)

func ingestEvent(ctx context.Context, db *sql.DB, ev *coachv1.Event) (*coachv1.EventResult, error) {

	// use the helper functions to validate and triage the payload
	if err := validateEvent(ev); err != nil {
		return result(ev, coachv1.IngestStatus_REJECTED, err.Error()), nil
	}

	r, err := routeTriage(ev)
	if err != nil {
		return nil, fmt.Errorf("routing event: %w", err)
	}
	// marshal the payload
	eventBytes, err := proto.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("marshalling event: %w", err)
	}
	var snapshotBytes []byte // stays nil for a delete, which pgx stores as NULL
	if r.entity != nil {
		snapshotBytes, err = proto.Marshal(r.entity)
		if err != nil {
			return nil, fmt.Errorf("marshalling snapshot: %w", err)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// if transaction holds the user, throw error
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, ev.GetUserId()); err != nil {
		return nil, fmt.Errorf("locking user: %w", err)
	}

	var isErased bool
	userHash := sha256.Sum256([]byte(ev.GetUserId()))

	//check if the account was deleted, if yes - reject
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM erased_users WHERE user_hash = $1)`, userHash[:]).Scan(&isErased)
	if err != nil {
		return nil, fmt.Errorf("checking erased users: %w", err)
	}
	if isErased {
		return result(ev, coachv1.IngestStatus_REJECTED, "user was erased"), nil
	}

	// check if the event is a duplicate
	res, err := tx.ExecContext(ctx, `
		INSERT INTO events (event_id, user_id, sequence, kind, occurred_at, payload)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (event_id) DO NOTHING`,
		ev.GetEventId(), ev.GetUserId(), ev.GetSequence(), r.eventKind, ev.GetOccurredAt().AsTime(), eventBytes,
	)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "events_user_id_sequence_key" {
		return result(ev, coachv1.IngestStatus_REJECTED, "sequence already used by another event"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("inserting event: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("reading inserted event count: %w", err)
	}
	if n == 0 {
		return result(ev, coachv1.IngestStatus_DUPLICATE, ""), nil
	}

	res, err = tx.ExecContext(ctx, `
		INSERT INTO snapshots (user_id, kind, aggregate_id, sequence, deleted, payload)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, kind, aggregate_id) DO UPDATE
		SET sequence = EXCLUDED.sequence, deleted = EXCLUDED.deleted, payload = EXCLUDED.payload
		WHERE snapshots.sequence < EXCLUDED.sequence`,
		ev.GetUserId(), r.snapshotKind, r.aggregateID, ev.GetSequence(), r.entity == nil, snapshotBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("upserting snapshot: %w", err)
	}
	n, err = res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("reading upserted snapshot count: %w", err)
	}
	status := coachv1.IngestStatus_APPLIED
	if n == 0 {
		status = coachv1.IngestStatus_SUPERSEDED
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing: %w", err)
	}
	return result(ev, status, ""), nil
}

func result(ev *coachv1.Event, status coachv1.IngestStatus, reason string) *coachv1.EventResult {
	return &coachv1.EventResult{EventId: ev.GetEventId(), Status: status, Reason: reason}
}
