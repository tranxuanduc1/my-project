CREATE TABLE IF NOT EXISTS iam.users (
  id uuid PRIMARY KEY,
  email text NOT NULL UNIQUE,
  password_hash text NOT NULL,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS iam.roles (
  id uuid PRIMARY KEY,
  name text NOT NULL UNIQUE,
  description text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS iam.user_roles (
  user_id uuid NOT NULL REFERENCES iam.users(id) ON DELETE CASCADE,
  role_id uuid NOT NULL REFERENCES iam.roles(id) ON DELETE RESTRICT,
  PRIMARY KEY (user_id, role_id)
);
INSERT INTO iam.roles(id,name,description) VALUES
 ('00000000-0000-0000-0000-000000000001','admin','System administrator'),
 ('00000000-0000-0000-0000-000000000002','customer','Customer')
ON CONFLICT (name) DO NOTHING;
