-- +goose Up
-- Phase 12c: health depth — exercise library, workout routines, personal
-- food database, meal slots, goal weight.

CREATE TABLE exercises (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    muscle_group TEXT NOT NULL DEFAULT '',
    equipment    TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL
);
CREATE INDEX idx_exercises_muscle ON exercises(muscle_group);

INSERT INTO exercises (id,name,muscle_group,equipment,created_at) VALUES
 ('ex_bench','Bench Press','chest','barbell','2026-01-01T00:00:00Z'),
 ('ex_incline_bench','Incline Bench Press','chest','barbell','2026-01-01T00:00:00Z'),
 ('ex_db_bench','Dumbbell Bench Press','chest','dumbbell','2026-01-01T00:00:00Z'),
 ('ex_pushup','Push-up','chest','bodyweight','2026-01-01T00:00:00Z'),
 ('ex_ohp','Overhead Press','shoulders','barbell','2026-01-01T00:00:00Z'),
 ('ex_lateral_raise','Lateral Raise','shoulders','dumbbell','2026-01-01T00:00:00Z'),
 ('ex_squat','Back Squat','legs','barbell','2026-01-01T00:00:00Z'),
 ('ex_front_squat','Front Squat','legs','barbell','2026-01-01T00:00:00Z'),
 ('ex_goblet_squat','Goblet Squat','legs','dumbbell','2026-01-01T00:00:00Z'),
 ('ex_deadlift','Deadlift','back','barbell','2026-01-01T00:00:00Z'),
 ('ex_rdl','Romanian Deadlift','hamstrings','barbell','2026-01-01T00:00:00Z'),
 ('ex_barbell_row','Barbell Row','back','barbell','2026-01-01T00:00:00Z'),
 ('ex_pullup','Pull-up','back','bodyweight','2026-01-01T00:00:00Z'),
 ('ex_lat_pulldown','Lat Pulldown','back','machine','2026-01-01T00:00:00Z'),
 ('ex_seated_cable_row','Seated Cable Row','back','machine','2026-01-01T00:00:00Z'),
 ('ex_leg_press','Leg Press','legs','machine','2026-01-01T00:00:00Z'),
 ('ex_lunge','Lunge','legs','dumbbell','2026-01-01T00:00:00Z'),
 ('ex_leg_curl','Leg Curl','hamstrings','machine','2026-01-01T00:00:00Z'),
 ('ex_leg_extension','Leg Extension','quads','machine','2026-01-01T00:00:00Z'),
 ('ex_calf_raise','Calf Raise','calves','machine','2026-01-01T00:00:00Z'),
 ('ex_barbell_curl','Barbell Curl','biceps','barbell','2026-01-01T00:00:00Z'),
 ('ex_db_curl','Dumbbell Curl','biceps','dumbbell','2026-01-01T00:00:00Z'),
 ('ex_hammer_curl','Hammer Curl','biceps','dumbbell','2026-01-01T00:00:00Z'),
 ('ex_tricep_pushdown','Tricep Pushdown','triceps','machine','2026-01-01T00:00:00Z'),
 ('ex_skullcrusher','Skullcrusher','triceps','barbell','2026-01-01T00:00:00Z'),
 ('ex_dips','Dips','triceps','bodyweight','2026-01-01T00:00:00Z'),
 ('ex_plank','Plank','core','bodyweight','2026-01-01T00:00:00Z'),
 ('ex_hanging_leg_raise','Hanging Leg Raise','core','bodyweight','2026-01-01T00:00:00Z'),
 ('ex_cable_crunch','Cable Crunch','core','machine','2026-01-01T00:00:00Z'),
 ('ex_running','Running','cardio','','2026-01-01T00:00:00Z'),
 ('ex_cycling','Cycling','cardio','','2026-01-01T00:00:00Z'),
 ('ex_rowing','Rowing Machine','cardio','machine','2026-01-01T00:00:00Z'),
 ('ex_jump_rope','Jump Rope','cardio','','2026-01-01T00:00:00Z');

CREATE TABLE routines (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    notes       TEXT NOT NULL DEFAULT '',
    tags        TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(tags) AND json_type(tags)='array'),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE routine_exercises (
    id          TEXT PRIMARY KEY,
    routine_id  TEXT NOT NULL REFERENCES routines(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL DEFAULT 0,
    name        TEXT NOT NULL,
    sets        INTEGER NOT NULL DEFAULT 3,
    target_reps INTEGER NOT NULL DEFAULT 10
);
CREATE INDEX idx_routine_exercises ON routine_exercises(routine_id, position);

CREATE TABLE foods (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    serving_desc TEXT NOT NULL DEFAULT '1 serving',
    calories     INTEGER NOT NULL DEFAULT 0,
    protein_g    REAL NOT NULL DEFAULT 0,
    carbs_g      REAL NOT NULL DEFAULT 0,
    fat_g        REAL NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL
);

ALTER TABLE meals ADD COLUMN slot TEXT CHECK(slot IS NULL OR slot IN ('breakfast','lunch','dinner','snack'));
ALTER TABLE health_settings ADD COLUMN goal_weight_kg REAL CHECK(goal_weight_kg IS NULL OR goal_weight_kg > 0);

-- +goose Down
ALTER TABLE health_settings DROP COLUMN goal_weight_kg;
ALTER TABLE meals DROP COLUMN slot;
DROP TABLE IF EXISTS foods;
DROP TABLE IF EXISTS routine_exercises;
DROP TABLE IF EXISTS routines;
DROP TABLE IF EXISTS exercises;
