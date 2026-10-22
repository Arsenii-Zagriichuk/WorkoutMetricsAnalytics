-- +goose Up
CREATE TABLE events (
    event_id    UUID        PRIMARY KEY,
    user_id     TEXT        NOT NULL,
    sequence    BIGINT      NOT NULL CHECK (sequence > 0),
    kind        TEXT        NOT NULL CHECK (kind IN (
                    'workout_upserted', 'workout_deleted', 'routine_upserted', 'profile_updated',
                    'body_weight_recorded', 'body_weight_removed', 'nutrition_day_updated', 'user_deleted'
                )),
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload     BYTEA       NOT NULL,
    UNIQUE (user_id, sequence)
);

CREATE TABLE snapshots (
    user_id      TEXT    NOT NULL,
    kind         TEXT    NOT NULL CHECK (kind IN (
                     'workout', 'routine', 'profile', 'body_weight', 'nutrition_day', 'account'
                 )),
    aggregate_id TEXT    NOT NULL,
    sequence     BIGINT  NOT NULL CHECK (sequence > 0),
    deleted      BOOLEAN NOT NULL DEFAULT false,
    payload      BYTEA,
    PRIMARY KEY (user_id, kind, aggregate_id),
    CHECK (deleted = (payload IS NULL))
);

CREATE TABLE erased_users (
    user_hash BYTEA       PRIMARY KEY CHECK (length(user_hash) = 32),
    erased_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE erased_users;
DROP TABLE snapshots;
DROP TABLE events;
