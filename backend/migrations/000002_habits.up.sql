CREATE TABLE habits (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_habits_user ON habits (user_id);

CREATE TABLE habit_checkins (
    habit_id   uuid NOT NULL REFERENCES habits (id) ON DELETE CASCADE,
    day        date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (habit_id, day)
);
