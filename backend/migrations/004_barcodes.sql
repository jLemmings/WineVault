ALTER TABLE bottles ADD COLUMN barcode text NOT NULL DEFAULT '' CHECK (barcode = '' OR barcode ~ '^([0-9]{8}|[0-9]{13})$');
CREATE INDEX bottles_barcode_idx ON bottles(barcode) WHERE barcode <> '';
