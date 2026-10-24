ALTER TABLE voucher_batches ADD COLUMN username_length INTEGER NOT NULL DEFAULT 8 CHECK(username_length BETWEEN 4 AND 32);
ALTER TABLE voucher_batches ADD COLUMN username_mode TEXT NOT NULL DEFAULT 'alphanumeric' CHECK(username_mode IN ('alphanumeric','letters','numbers'));
ALTER TABLE voucher_batches ADD COLUMN password_mode TEXT NOT NULL DEFAULT 'none' CHECK(password_mode IN ('none','same','random'));
ALTER TABLE voucher_batches ADD COLUMN password_length INTEGER NOT NULL DEFAULT 4 CHECK(password_length BETWEEN 4 AND 32);
ALTER TABLE voucher_batches ADD COLUMN password_charset TEXT NOT NULL DEFAULT 'alphanumeric' CHECK(password_charset IN ('alphanumeric','letters','numbers'));

ALTER TABLE vouchers ADD COLUMN password TEXT NOT NULL DEFAULT '';
