package coach

import (
	coachv1 "WorkoutMetricsAnalytics/gen/levl/coach/v1"
	"WorkoutMetricsAnalytics/internal/store"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/protobuf/proto"
)

// testDB connects to TEST_DATABASE_URL, migrates it and empties the coach tables.
// It refuses any database whose name doesn't contain "test", because it truncates.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set, e.g. postgres://<user>@localhost:5432/coach_test?sslmode=disable")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	var name string
	if err := db.QueryRow(`SELECT current_database()`).Scan(&name); err != nil {
		t.Fatalf("connecting to the test database: %v", err)
	}
	if !strings.Contains(name, "test") {
		t.Fatalf("refusing to truncate %q: TEST_DATABASE_URL must point at a test database", name)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	if _, err := db.Exec(`TRUNCATE events, snapshots, erased_users`); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	return db
}

var lastEventID atomic.Int64

// at gives ev the sequence seq and an event_id no other event in this test run has.
func at(ev *coachv1.Event, seq int64) *coachv1.Event {
	ev.EventId = fmt.Sprintf("00000000-0000-4000-8000-%012d", lastEventID.Add(1))
	ev.Sequence = seq
	return ev
}

func ingest(t *testing.T, db *sql.DB, ev *coachv1.Event) *coachv1.EventResult {
	t.Helper()
	res, err := ingestEvent(context.Background(), db, ev)
	if err != nil {
		t.Fatalf("ingestEvent: %v", err)
	}
	return res
}

func wantStatus(t *testing.T, res *coachv1.EventResult, want coachv1.IngestStatus) {
	t.Helper()
	if res.GetStatus() != want {
		t.Errorf("status = %v (reason %q), want %v", res.GetStatus(), res.GetReason(), want)
	}
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

type snapshotRow struct {
	found    bool
	sequence int64
	deleted  bool
	payload  []byte
}

func readSnapshot(t *testing.T, db *sql.DB, userID, kind, aggregateID string) snapshotRow {
	t.Helper()
	var s snapshotRow
	err := db.QueryRow(
		`SELECT sequence, deleted, payload FROM snapshots WHERE user_id = $1 AND kind = $2 AND aggregate_id = $3`,
		userID, kind, aggregateID,
	).Scan(&s.sequence, &s.deleted, &s.payload)
	if errors.Is(err, sql.ErrNoRows) {
		return s
	}
	if err != nil {
		t.Fatal(err)
	}
	s.found = true
	return s
}

func snapshotWorkout(t *testing.T, s snapshotRow) *coachv1.Workout {
	t.Helper()
	w := &coachv1.Workout{}
	if err := proto.Unmarshal(s.payload, w); err != nil {
		t.Fatalf("unmarshalling the workout snapshot: %v", err)
	}
	return w
}

func benchAt(weightKg float64) *coachv1.Event {
	return workoutEvent(func(w *coachv1.Workout) { w.Exercises[0].Sets[0].WeightKg = ptr(weightKg) })
}

const (
	testUser    = "clx9k2abcdefghijklmnopqrs"
	testWorkout = "clxaworkout1"
)

func TestIngestAppliesANewWorkout(t *testing.T) {
	db := testDB(t)
	ev := at(event(nil), 5)

	res := ingest(t, db, ev)
	wantStatus(t, res, coachv1.IngestStatus_APPLIED)
	if res.GetEventId() != ev.GetEventId() {
		t.Errorf("result event_id = %q, want %q", res.GetEventId(), ev.GetEventId())
	}

	var kind string
	var payload []byte
	if err := db.QueryRow(`SELECT kind, payload FROM events WHERE event_id = $1`, ev.GetEventId()).Scan(&kind, &payload); err != nil {
		t.Fatal(err)
	}
	if kind != "workout_upserted" {
		t.Errorf("events.kind = %q, want workout_upserted", kind)
	}
	stored := &coachv1.Event{}
	if err := proto.Unmarshal(payload, stored); err != nil || !proto.Equal(stored, ev) {
		t.Errorf("events.payload doesn't hold the whole event (err %v)", err)
	}

	s := readSnapshot(t, db, testUser, "workout", testWorkout)
	if !s.found || s.sequence != 5 || s.deleted {
		t.Fatalf("snapshot = %+v, want a live row at sequence 5", s)
	}
	if !proto.Equal(snapshotWorkout(t, s), ev.GetWorkoutUpserted().GetWorkout()) {
		t.Error("the snapshot doesn't hold the workout that was sent")
	}
}

func TestIngestSameEventTwiceIsADuplicate(t *testing.T) {
	db := testDB(t)
	ev := at(event(nil), 5)
	ingest(t, db, ev)
	before := readSnapshot(t, db, testUser, "workout", testWorkout)

	wantStatus(t, ingest(t, db, ev), coachv1.IngestStatus_DUPLICATE)

	if n := countRows(t, db, "events"); n != 1 {
		t.Errorf("events rows = %d, want 1", n)
	}
	if after := readSnapshot(t, db, testUser, "workout", testWorkout); after.sequence != before.sequence {
		t.Errorf("snapshot moved from sequence %d to %d", before.sequence, after.sequence)
	}
}

func TestIngestOlderSequenceIsSuperseded(t *testing.T) {
	db := testDB(t)
	ingest(t, db, at(benchAt(100), 5))

	wantStatus(t, ingest(t, db, at(benchAt(50), 3)), coachv1.IngestStatus_SUPERSEDED)

	s := readSnapshot(t, db, testUser, "workout", testWorkout)
	if s.sequence != 5 {
		t.Errorf("snapshot sequence = %d, want 5", s.sequence)
	}
	if got := snapshotWorkout(t, s).GetExercises()[0].GetSets()[0].GetWeightKg(); got != 100 {
		t.Errorf("snapshot weight = %v, want 100 from sequence 5", got)
	}
	// Kept, so that a redelivery of it is caught as a duplicate.
	if n := countRows(t, db, "events"); n != 2 {
		t.Errorf("events rows = %d, want 2", n)
	}
}

func TestIngestNewerSequenceReplacesTheSnapshot(t *testing.T) {
	db := testDB(t)
	ingest(t, db, at(benchAt(50), 3))

	wantStatus(t, ingest(t, db, at(benchAt(100), 5)), coachv1.IngestStatus_APPLIED)

	s := readSnapshot(t, db, testUser, "workout", testWorkout)
	if got := snapshotWorkout(t, s).GetExercises()[0].GetSets()[0].GetWeightKg(); s.sequence != 5 || got != 100 {
		t.Errorf("snapshot = sequence %d, weight %v; want sequence 5, weight 100", s.sequence, got)
	}
}

func TestIngestDeleteLeavesATombstoneThatOlderUpsertsCantUndo(t *testing.T) {
	db := testDB(t)
	ingest(t, db, at(event(nil), 5))

	wantStatus(t, ingest(t, db, at(workoutDeletedEvent(testWorkout), 7)), coachv1.IngestStatus_APPLIED)
	s := readSnapshot(t, db, testUser, "workout", testWorkout)
	if !s.deleted || s.payload != nil || s.sequence != 7 {
		t.Fatalf("snapshot = %+v, want a tombstone at sequence 7 with no payload", s)
	}

	wantStatus(t, ingest(t, db, at(event(nil), 6)), coachv1.IngestStatus_SUPERSEDED)
	if s := readSnapshot(t, db, testUser, "workout", testWorkout); !s.deleted {
		t.Error("an older upsert brought the deleted workout back")
	}
}

func TestIngestRejectsAReusedSequence(t *testing.T) {
	db := testDB(t)
	ingest(t, db, at(event(nil), 5))

	res := ingest(t, db, at(bodyWeightEvent("2026-10-22", 80), 5))
	wantStatus(t, res, coachv1.IngestStatus_REJECTED)
	if !strings.Contains(res.GetReason(), "sequence") {
		t.Errorf("reason = %q, want it to name the sequence", res.GetReason())
	}
	if n := countRows(t, db, "events"); n != 1 {
		t.Errorf("events rows = %d, want 1", n)
	}
	if s := readSnapshot(t, db, testUser, "body_weight", "2026-10-22"); s.found {
		t.Error("the rejected event still wrote a snapshot")
	}
}

func TestIngestRejectsAnErasedUser(t *testing.T) {
	db := testDB(t)
	hash := sha256.Sum256([]byte(testUser))
	if _, err := db.Exec(`INSERT INTO erased_users (user_hash) VALUES ($1)`, hash[:]); err != nil {
		t.Fatal(err)
	}

	res := ingest(t, db, at(event(nil), 1))
	wantStatus(t, res, coachv1.IngestStatus_REJECTED)
	if !strings.Contains(res.GetReason(), "erased") {
		t.Errorf("reason = %q, want it to say the user was erased", res.GetReason())
	}
	if n := countRows(t, db, "events") + countRows(t, db, "snapshots"); n != 0 {
		t.Errorf("%d rows written for an erased user, want 0", n)
	}
}

func TestIngestRejectsAnInvalidEventWithoutWriting(t *testing.T) {
	db := testDB(t)

	res := ingest(t, db, at(workoutEvent(func(w *coachv1.Workout) { w.WorkoutId = "" }), 1))
	wantStatus(t, res, coachv1.IngestStatus_REJECTED)
	if !strings.Contains(res.GetReason(), "workout.workout_id") {
		t.Errorf("reason = %q, want the validation message", res.GetReason())
	}
	if n := countRows(t, db, "events") + countRows(t, db, "snapshots"); n != 0 {
		t.Errorf("%d rows written for an invalid event, want 0", n)
	}
}

// Every kind has to get past the migration's CHECK lists. The final comparison against
// the schema makes a new event type fail here until it's added to this list.
func TestIngestAppliesEveryPayloadType(t *testing.T) {
	db := testDB(t)
	events := []*coachv1.Event{
		event(nil),
		workoutDeletedEvent("clxaworkout2"),
		routineEvent(&coachv1.RoutineExercise{ExerciseId: "bench", TargetSets: 3}),
		profileEvent(nil),
		bodyWeightEvent("2026-10-22", 82.5),
		bodyWeightRemovedEvent("2026-10-21"),
		nutritionEvent(&coachv1.NutritionDayUpdated{Date: "2026-10-22", Calories: 2400}),
		userDeletedEvent(),
	}
	for i, ev := range events {
		res := ingest(t, db, at(ev, int64(i+1)))
		if res.GetStatus() != coachv1.IngestStatus_APPLIED {
			t.Errorf("event %d: status = %v (reason %q), want APPLIED", i, res.GetStatus(), res.GetReason())
		}
	}

	var stored []string
	rows, err := db.Query(`SELECT DISTINCT kind FROM events`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		stored = append(stored, k)
	}
	payload := (&coachv1.Event{}).ProtoReflect().Descriptor().Oneofs().ByName("payload")
	for i := 0; i < payload.Fields().Len(); i++ {
		if name := string(payload.Fields().Get(i).Name()); !slices.Contains(stored, name) {
			t.Errorf("no %s event was ingested; add one to this test", name)
		}
	}
}

func TestIngestKeepsUsersApart(t *testing.T) {
	db := testDB(t)
	other := func(ev *coachv1.Event) *coachv1.Event { ev.UserId = "clx9otheruser000000000000"; return ev }
	ingest(t, db, at(bodyWeightEvent("2026-10-22", 80), 1))
	ingest(t, db, at(other(bodyWeightEvent("2026-10-22", 95)), 1))

	ingest(t, db, at(other(bodyWeightRemovedEvent("2026-10-22")), 2))

	if s := readSnapshot(t, db, testUser, "body_weight", "2026-10-22"); !s.found || s.deleted {
		t.Errorf("removing another user's weigh-in changed this user's: %+v", s)
	}
	if s := readSnapshot(t, db, "clx9otheruser000000000000", "body_weight", "2026-10-22"); !s.deleted {
		t.Errorf("the other user's weigh-in should be a tombstone: %+v", s)
	}
}

// 50 versions of one workout arrive at once, in random order. Nothing in Go guards
// this: the per-user lock and the WHERE on the upsert must leave the newest.
func TestIngestConcurrentVersionsKeepTheNewest(t *testing.T) {
	db := testDB(t)
	const versions = 50
	events := make([]*coachv1.Event, versions)
	for i, p := range rand.Perm(versions) {
		seq := int64(p + 1)
		events[i] = at(benchAt(float64(seq)), seq)
	}

	results := make([]*coachv1.EventResult, versions)
	errs := make([]error, versions)
	var wg sync.WaitGroup
	for i, ev := range events {
		wg.Go(func() { results[i], errs[i] = ingestEvent(context.Background(), db, ev) })
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
	}
	for i, res := range results {
		if s := res.GetStatus(); s != coachv1.IngestStatus_APPLIED && s != coachv1.IngestStatus_SUPERSEDED {
			t.Errorf("event %d: status = %v (reason %q)", i, s, res.GetReason())
		}
	}
	if n := countRows(t, db, "events"); n != versions {
		t.Errorf("events rows = %d, want %d", n, versions)
	}
	s := readSnapshot(t, db, testUser, "workout", testWorkout)
	if got := snapshotWorkout(t, s).GetExercises()[0].GetSets()[0].GetWeightKg(); s.sequence != versions || got != versions {
		t.Errorf("snapshot = sequence %d, weight %v; want both %d", s.sequence, got, versions)
	}
}
