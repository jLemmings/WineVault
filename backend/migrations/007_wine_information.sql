CREATE TABLE wine_information (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 name text NOT NULL,
 vintage integer NOT NULL,
 region text NOT NULL,
 wine_type text NOT NULL,
 status text NOT NULL DEFAULT 'not_fetched',
 source_id bigint,
 payload jsonb,
 fetched_at timestamptz,
 attempted_at timestamptz,
 message text NOT NULL DEFAULT '',
 UNIQUE(name,vintage,region,wine_type)
);
ALTER TABLE bottles ADD COLUMN information_id bigint REFERENCES wine_information(id);
INSERT INTO wine_information(name,vintage,region,wine_type)
SELECT DISTINCT name,vintage,region,wine_type FROM bottles;
UPDATE bottles b SET information_id=i.id FROM wine_information i
WHERE (b.name,b.vintage,b.region,b.wine_type)=(i.name,i.vintage,i.region,i.wine_type);
CREATE FUNCTION attach_wine_information() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO wine_information(name,vintage,region,wine_type)
 VALUES(NEW.name,NEW.vintage,NEW.region,NEW.wine_type)
 ON CONFLICT(name,vintage,region,wine_type) DO UPDATE SET name=EXCLUDED.name
 RETURNING id INTO NEW.information_id;
 RETURN NEW;
END;
$$;
CREATE TRIGGER bottle_information BEFORE INSERT OR UPDATE OF name,vintage,region,wine_type ON bottles
FOR EACH ROW EXECUTE FUNCTION attach_wine_information();
