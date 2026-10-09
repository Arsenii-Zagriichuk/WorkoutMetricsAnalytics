-- +goose Up
CREATE TABLE workouts (
                          id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
                          user_id           TEXT        NOT NULL,
                          client_workout_id INTEGER     NOT NULL CHECK (client_workout_id >= 1),
                          performed_at      TIMESTAMPTZ NOT NULL,
                          created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
                          UNIQUE (user_id, client_workout_id)
);
CREATE INDEX workouts_user_performed_at_idx ON workouts (user_id, performed_at);

CREATE TABLE exercises (
                           id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
                           workout_id BIGINT  NOT NULL REFERENCES workouts (id) ON DELETE CASCADE,
                           position   INTEGER NOT NULL,
                           name       TEXT    NOT NULL,
                           UNIQUE (workout_id, position)
);

CREATE TABLE sets (
                      exercise_id    BIGINT           NOT NULL REFERENCES exercises (id) ON DELETE CASCADE,
                      position       INTEGER          NOT NULL,
                      reps           INTEGER          NOT NULL CHECK (reps >= 1),
                      working_weight DOUBLE PRECISION NOT NULL CHECK (working_weight >= 0),
                      PRIMARY KEY (exercise_id, position)
);

-- +goose Down
DROP TABLE sets;
DROP TABLE exercises;
DROP TABLE workouts;