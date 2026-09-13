ALTER TABLE bottles DROP CONSTRAINT bottles_vintage_check;
ALTER TABLE bottles ADD CONSTRAINT bottles_vintage_check CHECK (vintage = 0 OR vintage >= 1900);
COMMENT ON COLUMN bottles.vintage IS 'Vintage year; 0 means explicitly non-vintage (NV). Unknown years must be reviewed before saving.';
