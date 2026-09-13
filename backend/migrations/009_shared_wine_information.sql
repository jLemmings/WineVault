-- Catalogue details describe a wine, not an individual bottle or vintage.
ALTER TABLE wine_information ADD COLUMN name_key text GENERATED ALWAYS AS (lower(regexp_replace(btrim(name), '\s+', ' ', 'g'))) STORED;
ALTER TABLE wine_information ADD COLUMN region_key text GENERATED ALWAYS AS (lower(regexp_replace(btrim(region), '\s+', ' ', 'g'))) STORED;
ALTER TABLE wine_information ADD COLUMN type_key text GENERATED ALWAYS AS (lower(btrim(wine_type))) STORED;

-- Prefer actual saved data, then the most recent successful fetch. Relink all
-- existing bottles before removing duplicate cache records.
CREATE TEMP TABLE information_merge ON COMMIT DROP AS
SELECT id, first_value(id) OVER (
 PARTITION BY name_key,region_key,type_key
 ORDER BY (payload IS NOT NULL AND fetched_at IS NOT NULL) DESC,
 fetched_at DESC NULLS LAST, attempted_at DESC NULLS LAST, id
) AS keep_id FROM wine_information;
UPDATE bottles b SET information_id=m.keep_id FROM information_merge m
WHERE b.information_id=m.id AND m.id<>m.keep_id;
DELETE FROM wine_information i USING information_merge m WHERE i.id=m.id AND m.id<>m.keep_id;
ALTER TABLE wine_information DROP CONSTRAINT wine_information_name_vintage_region_wine_type_key;
CREATE UNIQUE INDEX wine_information_identity ON wine_information(name_key,region_key,type_key);

CREATE OR REPLACE FUNCTION attach_wine_information() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO wine_information(name,vintage,region,wine_type)
 VALUES(NEW.name,NEW.vintage,NEW.region,NEW.wine_type)
 ON CONFLICT(name_key,region_key,type_key) DO UPDATE SET name=wine_information.name
 RETURNING id INTO NEW.information_id;
 RETURN NEW;
END;
$$;
