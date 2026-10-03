ALTER TABLE cellars ADD COLUMN preferences jsonb NOT NULL DEFAULT '{"viewMode":"floor-plan","typeRacks":{}}';
ALTER TABLE bottles DROP CONSTRAINT bottles_wine_type_check;
ALTER TABLE bottles ADD CONSTRAINT bottles_wine_type_check CHECK(wine_type IN ('Red','White','Rosé','Champagne','Sparkling','Dessert'));
