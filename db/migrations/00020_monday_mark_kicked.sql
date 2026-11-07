-- +goose Up

-- A mentor marks the week a student was kicked as "kicked", and explains why
-- in their notes. It is never set automatically.
ALTER TYPE app.monday_update_mark ADD VALUE IF NOT EXISTS 'kicked';

-- +goose Down
-- PostgreSQL cannot drop an enum value; marks of "kicked" become "at_risk".
UPDATE app.monday_update_marks SET mark = 'at_risk' WHERE mark = 'kicked';
UPDATE app.monday_week_marks SET mark = 'at_risk' WHERE mark = 'kicked';
