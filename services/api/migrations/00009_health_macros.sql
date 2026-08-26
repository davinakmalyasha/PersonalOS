-- +goose Up
-- Phase 10c: meal macros, free-form body measurements, health settings.

ALTER TABLE meals ADD COLUMN protein_g REAL CHECK(protein_g IS NULL OR protein_g >= 0);
ALTER TABLE meals ADD COLUMN carbs_g   REAL CHECK(carbs_g   IS NULL OR carbs_g   >= 0);
ALTER TABLE meals ADD COLUMN fat_g     REAL CHECK(fat_g     IS NULL OR fat_g     >= 0);

ALTER TABLE body_metrics ADD COLUMN measurements TEXT NOT NULL DEFAULT '{}'
    CHECK(json_valid(measurements) AND json_type(measurements)='object');

CREATE TABLE health_settings (
    id                    TEXT PRIMARY KEY,
    calorie_target        INTEGER CHECK(calorie_target        IS NULL OR calorie_target        >= 0),
    protein_target_g      INTEGER CHECK(protein_target_g      IS NULL OR protein_target_g      >= 0),
    carbs_target_g        INTEGER CHECK(carbs_target_g        IS NULL OR carbs_target_g        >= 0),
    fat_target_g          INTEGER CHECK(fat_target_g          IS NULL OR fat_target_g          >= 0),
    water_target_ml       INTEGER CHECK(water_target_ml       IS NULL OR water_target_ml       >= 0),
    weekly_workout_target INTEGER CHECK(weekly_workout_target IS NULL OR (weekly_workout_target BETWEEN 1 AND 14)),
    updated_at            TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS health_settings;
ALTER TABLE body_metrics DROP COLUMN measurements;
ALTER TABLE meals DROP COLUMN fat_g;
ALTER TABLE meals DROP COLUMN carbs_g;
ALTER TABLE meals DROP COLUMN protein_g;
