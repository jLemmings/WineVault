ALTER TABLE bottles ADD COLUMN revision integer NOT NULL DEFAULT 1;
CREATE FUNCTION bump_bottle_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.revision := OLD.revision + 1; RETURN NEW; END;
$$;
CREATE TRIGGER bottle_revision BEFORE UPDATE ON bottles FOR EACH ROW EXECUTE FUNCTION bump_bottle_revision();
CREATE TABLE drinking_windows (
 wine_key text NOT NULL, vintage integer NOT NULL,
 start_year integer NOT NULL CHECK(start_year BETWEEN 1900 AND 9999),
 end_year integer NOT NULL CHECK(end_year BETWEEN start_year AND 9999),
 PRIMARY KEY(wine_key,vintage)
);
CREATE FUNCTION bottle_wine_key(text,text,text) RETURNS text LANGUAGE sql IMMUTABLE AS $$
 SELECT lower(regexp_replace(trim($1), '\s+', ' ', 'g')) || '|' || lower(regexp_replace(trim($2), '\s+', ' ', 'g')) || '|' || lower($3);
$$;
CREATE TABLE enjoyed_bottles (id bigint PRIMARY KEY, snapshot jsonb NOT NULL);
ALTER TABLE wine_history DROP CONSTRAINT wine_history_action_check;
ALTER TABLE wine_history ADD CHECK(action IN ('added','enjoyed','restored'));
CREATE OR REPLACE FUNCTION record_wine_history() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE b bottles%ROWTYPE; activity text;
BEGIN
 IF TG_OP = 'INSERT' THEN
  b := NEW;
  activity := CASE WHEN current_setting('winevault.restoring',true)='yes' THEN 'restored' ELSE 'added' END;
 ELSE
  b := OLD; activity := 'enjoyed';
  INSERT INTO enjoyed_bottles(id,snapshot) VALUES(b.id,to_jsonb(b)) ON CONFLICT(id) DO UPDATE SET snapshot=excluded.snapshot;
 END IF;
 INSERT INTO wine_history(action,bottle_id,name,vintage,region,wine_type,rack_id,rack_name,slot,columns)
 SELECT activity,b.id,b.name,b.vintage,b.region,b.wine_type,b.rack_id,r.name,b.slot,r.columns FROM racks r WHERE r.id=b.rack_id;
 RETURN b;
END;
$$;
