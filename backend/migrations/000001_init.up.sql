-- Users -----------------------------------------------------------------------
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name          text NOT NULL,
    email         text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Tasks -----------------------------------------------------------------------
CREATE TYPE task_priority AS ENUM ('low', 'medium', 'high');
CREATE TYPE task_status   AS ENUM ('pending', 'completed');

CREATE TABLE tasks (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title            text NOT NULL,
    description      text NOT NULL DEFAULT '',
    priority         task_priority NOT NULL DEFAULT 'medium',
    duration_minutes integer NOT NULL DEFAULT 30 CHECK (duration_minutes > 0),
    status           task_status NOT NULL DEFAULT 'pending',
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_tasks_user_id     ON tasks (user_id);
CREATE INDEX idx_tasks_user_status ON tasks (user_id, status);

-- Daily plans (populated in Batch 4) ------------------------------------------
CREATE TYPE plan_status AS ENUM ('queued', 'running', 'done', 'failed');

CREATE TABLE daily_plans (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    plan_date  date NOT NULL,
    status     plan_status NOT NULL DEFAULT 'queued',
    plan_json  jsonb,
    error      text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, plan_date)
);

CREATE INDEX idx_daily_plans_user_date ON daily_plans (user_id, plan_date);
