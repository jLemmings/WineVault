ALTER TABLE bottles
    ADD COLUMN price_minor bigint CHECK (price_minor BETWEEN 0 AND 1000000000000);

ALTER TABLE bottles
    ADD COLUMN currency text NOT NULL DEFAULT '' CHECK(currency IN ('', 'CHF', 'EUR', 'USD', 'GBP', 'CAD', 'AUD'));

ALTER TABLE bottles
    ADD COLUMN purchased_on date;

ALTER TABLE bottles
    ADD COLUMN seller text NOT NULL DEFAULT '' CHECK (length(seller) <= 200);

ALTER TABLE bottles
    ADD CHECK (price_minor IS NULL
        OR currency <> '');

CREATE TABLE wine_purchases (
    bottle_id bigint PRIMARY KEY,
    name text NOT NULL,
    vintage integer NOT NULL,
    price_minor bigint,
    currency text NOT NULL,
    purchased_on date,
    seller text NOT NULL
);

CREATE FUNCTION record_wine_purchase ()
    RETURNS TRIGGER
    LANGUAGE plpgsql
    AS $$
BEGIN
    INSERT INTO wine_purchases (bottle_id, name, vintage, price_minor, currency, purchased_on, seller)
        VALUES (NEW.id, NEW.name, NEW.vintage, NEW.price_minor, NEW.currency, NEW.purchased_on, NEW.seller)
    ON CONFLICT (bottle_id)
        DO UPDATE SET
            name = excluded.name, vintage = excluded.vintage, price_minor = excluded.price_minor, currency = excluded.currency, purchased_on = excluded.purchased_on, seller = excluded.seller;
    RETURN NEW;
END;
$$;

CREATE TRIGGER wine_purchase_changes
    AFTER INSERT OR UPDATE ON bottles
    FOR EACH ROW
    EXECUTE FUNCTION record_wine_purchase ();

INSERT INTO wine_purchases
SELECT
    id,
    name,
    vintage,
    price_minor,
    currency,
    purchased_on,
    seller
FROM
    bottles;
