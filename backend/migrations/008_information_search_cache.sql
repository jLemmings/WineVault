ALTER TABLE wine_information ADD COLUMN candidates jsonb;
ALTER TABLE wine_information ADD COLUMN search_query text NOT NULL DEFAULT '';
