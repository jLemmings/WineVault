INSERT INTO bottles (name,vintage,region,wine_type,rack_id,slot)
SELECT name,2015 + slot % 6,region,wine_type,rack_id,slot
FROM (VALUES
 ('Château Margaux','Bordeaux, France','Red','A',28),
 ('Domaine de la Romanée-Conti','Burgundy, France','Red','B',24),
 ('Dom Pérignon','Champagne, France','Sparkling','C',18),
 ('Château d’Yquem','Sauternes, France','White','D',12)
) AS defaults(name,region,wine_type,rack_id,quantity)
CROSS JOIN LATERAL generate_series(0,quantity-1) AS slot;
