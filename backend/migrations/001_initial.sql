CREATE TABLE cellars (
 id text PRIMARY KEY,
 name text NOT NULL,
 owner_name text NOT NULL,
 room_name text NOT NULL,
 width_m numeric(6,2) NOT NULL CHECK (width_m > 0),
 depth_m numeric(6,2) NOT NULL CHECK (depth_m > 0)
);
CREATE TABLE racks (
 id text PRIMARY KEY,
 cellar_id text NOT NULL REFERENCES cellars(id),
 name text NOT NULL,
 short_name text NOT NULL,
 wall text NOT NULL,
 grapes text NOT NULL,
 temperature integer NOT NULL,
 capacity integer NOT NULL CHECK (capacity > 0),
 color text NOT NULL,
 position integer NOT NULL UNIQUE
);
CREATE TABLE rack_slots (
 rack_id text NOT NULL REFERENCES racks(id),
 slot integer NOT NULL CHECK (slot >= 0),
 PRIMARY KEY (rack_id, slot)
);
CREATE TABLE bottles (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 name text NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 150),
 vintage integer NOT NULL CHECK (vintage >= 1900),
 region text NOT NULL CHECK (length(trim(region)) BETWEEN 1 AND 200),
 wine_type text NOT NULL CHECK (wine_type IN ('Red','White','Rosé','Sparkling','Dessert')),
 rack_id text NOT NULL,
 slot integer NOT NULL,
 FOREIGN KEY (rack_id, slot) REFERENCES rack_slots(rack_id, slot),
 UNIQUE (rack_id, slot)
);
INSERT INTO cellars VALUES ('grand-cru','Your cellar','Personal cellar','Cellar',5.40,4.20);
