# WorkoutMetricsAnalytics

A Go HTTP API that stores each user's workouts and computes their weekly training volume and personal records. Built with Gin, authenticated with JWTs. Workouts live in memory, so they are lost when the server restarts.

## Run it

Needs Go 1.26.

```bash
cp .env.example .env    # then set JWT_SECRET, e.g. from: openssl rand -base64 32
go run ./cmd/api        # listens on :8080
```

Without a `.env` file, the server reads `JWT_SECRET` from the environment.

## Get a token

There is no sign-up. The API accepts any HS256 token signed with `JWT_SECRET` that has issuer `workout-metrics-analytics`, an expiry, and a `sub` claim holding the user id. `devtoken` mints one:

```bash
TOKEN=$(go run ./cmd/devtoken -sub user-a)    # -ttl 2h for a longer-lived token
```

## Endpoints

Every route needs `Authorization: Bearer <token>` and only ever sees the caller's own workouts.

| Method | Path                               | Returns                                                                 |
| ------ | ---------------------------------- | ----------------------------------------------------------------------- |
| POST   | `/workouts`                        | Saves a workout                                                         |
| GET    | `/workouts/volume?exercise=<name>` | Volume (reps × weight) per ISO week                                     |
| GET    | `/workouts/pr?exercise=<name>`     | Best estimated one-rep max, heaviest weight, and best single-set volume |

### Save workouts

```bash
curl -X POST localhost:8080/workouts \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"workout_id":1,"date":"2026-09-28T10:00:00Z","exercises":[{"name":"bench press","sets":[{"reps":5,"working_weight":100},{"reps":5,"working_weight":100}]}]}'
# 201 {"message":"workout saved successfully","session_id":1}

curl -X POST localhost:8080/workouts \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"workout_id":2,"date":"2026-10-01T18:30:00Z","exercises":[{"name":"bench press","sets":[{"reps":3,"working_weight":110}]}]}'
# 201 {"message":"workout saved successfully","session_id":2}
```

A workout needs a `workout_id` of at least 1 that the user hasn't used before, a `date`, and at least one exercise. Each exercise needs a `name` and at least one set, and each set needs `reps` of at least 1 and a `working_weight` of at least 0. Anything else gets a 400 and isn't saved.

### Weekly volume

```bash
curl -G localhost:8080/workouts/volume --data-urlencode 'exercise=bench press' \
  -H "Authorization: Bearer $TOKEN"
# 200 {"2026-W40":1330}
```

Keys are ISO weeks, so a workout on 1 January 2027 counts towards `2026-W53`.

### Personal records

```bash
curl -G localhost:8080/workouts/pr --data-urlencode 'exercise=bench press' \
  -H "Authorization: Bearer $TOKEN"
# 200 {"one_rep_max":121.00000000000001,"weight":110,"volume":500}
```

The one-rep max is estimated with the Epley formula, weight × (1 + reps / 30), except that a single rep counts as is.

### Errors

Errors come back as `{"error": "<message>"}`.

| Status | When                                                                                         |
| ------ | -------------------------------------------------------------------------------------------- |
| 400    | The body isn't a valid workout (see Save workouts), or the `exercise` parameter is missing   |
| 401    | The token is missing, malformed, expired or wrongly signed (with `WWW-Authenticate: Bearer`) |
| 404    | The caller has never logged that exercise                                                    |
| 409    | The caller already saved a workout with that `workout_id`; the new one isn't saved           |
| 500    | Something failed on the server; the details are logged, not sent                             |

## Tests

```bash
go test -race ./...
```

- `internal/store`: 50 goroutines save and read one user's workouts at the same time, and every save must land; 50 goroutines saving the same `workout_id` at once store it exactly once.
- `internal/middleware`: `VerifyToken` rejects expired tokens, the wrong secret, HS512, a missing `sub` and the wrong issuer.
- `internal/services`: the one-rep-max estimate, and a bodyweight exercise with all-zero PRs still counting as logged.
- `cmd/api`: through the real router, user B can't read user A's workouts on any read route, invalid workouts get a 400 and aren't saved, the PR response has the documented shape, an exercise that was never logged gets a 404 from both read routes and a missing `exercise` a 400, and a repeated `workout_id` gets a 409 without being counted.

CI (`.github/workflows/action.yml`) runs `go vet`, a `gofmt` check and the tests with the race detector.

### The data race the mutex fixes

The store was a bare map before commit `8fd8eb3`. The concurrent test arrived in `d42d7f3`, before the store's methods took a context, so the demo runs there. On a clean checkout, this runs that test against the bare-map store and then comes back:

```bash
git switch --detach d42d7f3
git show 8fd8eb3~1:internal/store/memory.go > internal/store/memory.go
go test -race ./internal/store
git checkout internal/store/memory.go
git switch -
```

In one run the race detector reported 7 data races, and only 29 of the 50 saves survived. There were two kinds of race: `GetWorkouts` reading the map while `SaveWorkout` wrote it, and two `SaveWorkout` calls appending to the same slice at once, which is how saves get lost. The first report:

```text
WARNING: DATA RACE
Read at 0x00c0000a4600 by goroutine 21:
  ...
  WorkoutMetricsAnalytics/internal/store.(*Storage).GetWorkouts()
      WorkoutMetricsAnalytics/internal/store/memory.go:25 +0x5c

Previous write at 0x00c0000a4600 by goroutine 8:
  ...
  WorkoutMetricsAnalytics/internal/store.(*Storage).SaveWorkout()
      WorkoutMetricsAnalytics/internal/store/memory.go:20 +0x84
```

Without `-race`, the same test crashed with `fatal error: concurrent map writes` in all 20 runs. The Go runtime checks maps for concurrent writes, but nothing at runtime catches the slice race; that takes the race detector.

## Layout

```text
cmd/api               server entry point and routes
cmd/devtoken          prints a signed token for local testing
internal/handlers     HTTP handlers
internal/middleware   JWT authentication
internal/services     weekly volume and personal-record calculations
internal/store        in-memory storage guarded by a sync.RWMutex
internal/models       workout and personal-record types
```
