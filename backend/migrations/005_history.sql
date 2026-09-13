CREATE TABLE wine_history (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 action text NOT NULL CHECK (action IN ('added','enjoyed')),
 occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 bottle_id bigint NOT NULL,
 name text NOT NULL,
 vintage integer NOT NULL,
 region text NOT NULL,
 wine_type text NOT NULL,
 rack_id text NOT NULL,
 rack_name text NOT NULL,
 slot integer NOT NULL,
 columns integer NOT NULL
);

CREATE FUNCTION record_wine_history() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE b bottles%ROWTYPE;
BEGIN
 IF TG_OP = 'INSERT' THEN b := NEW; ELSE b := OLD; END IF;
 INSERT INTO wine_history(action,bottle_id,name,vintage,region,wine_type,rack_id,rack_name,slot,columns)
 SELECT CASE WHEN TG_OP='INSERT' THEN 'added' ELSE 'enjoyed' END,
 b.id,b.name,b.vintage,b.region,b.wine_type,b.rack_id,r.name,b.slot,r.columns
 FROM racks r WHERE r.id=b.rack_id;
 RETURN b;
END;
$$;
CREATE TRIGGER wine_history_changes AFTER INSERT OR DELETE ON bottles
FOR EACH ROW EXECUTE FUNCTION record_wine_history();
