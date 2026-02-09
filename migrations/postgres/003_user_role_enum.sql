DO $$
BEGIN
  CREATE TYPE user_role AS ENUM ('user', 'admin', 'super_admin');
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

UPDATE users
SET role = 'user'
WHERE role IS NULL
   OR role = ''
   OR role NOT IN ('user', 'admin', 'super_admin');

ALTER TABLE users
  ALTER COLUMN role TYPE user_role USING role::user_role,
  ALTER COLUMN role SET DEFAULT 'user',
  ALTER COLUMN role SET NOT NULL;
