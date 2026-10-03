INSERT INTO cellars VALUES ('grand-cru','Grand Cru Cellar','Jamie Dupont','VAULT A1',5.40,4.20);
INSERT INTO racks VALUES
 ('A','grand-cru','Bordeaux & Haut-Médoc','Bordeaux','North wall','Cabernet Sauvignon, Merlot & Bordeaux blends',14,30,'red',1),
 ('B','grand-cru','Burgundy & Rhône','Burgundy & Rhône','East wall','Pinot Noir, Syrah & Grenache blends',13,30,'red',2),
 ('C','grand-cru','Champagne & Whites','Champagne & Whites','South wall','Champagne, Riesling & Chardonnay',10,30,'white',3),
 ('D','grand-cru','The Reserve','The Reserve','West wall','Special vintages & sweet wines',12,20,'gold',4);
INSERT INTO rack_slots SELECT id, generate_series(0, capacity - 1) FROM racks;
