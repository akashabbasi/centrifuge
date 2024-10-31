CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  username VARCHAR(50) UNIQUE NOT NULL,
  password TEXT NOT NULL,
  role VARCHAR(20) NOT NULL DEFAULT 'user',  -- Add role column with a default value
  created_at TIMESTAMP DEFAULT NOW()
);


-- Casbin will create the necessary policy table automatically.
CREATE TABLE IF NOT EXISTS casbin_rule (
    id SERIAL PRIMARY KEY,
    ptype VARCHAR(100) NOT NULL,
    v0 VARCHAR(100),
    v1 VARCHAR(100),
    v2 VARCHAR(100),
    v3 VARCHAR(100),
    v4 VARCHAR(100),
    v5 VARCHAR(100)
);

-- Admin role can subscribe to the admin_channel
INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES ('p', 'admin', 'admin_channel', 'subscribe');

-- User role can subscribe to the user_channel
INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES ('p', 'user', 'user_channel', 'subscribe');

