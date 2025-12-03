-- Create keycloak user and database on first init
-- This script runs once when the postgres container starts with an empty data directory

DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'keycloak') THEN
    CREATE ROLE keycloak LOGIN PASSWORD 'keycloak';
  END IF;
END $$;

-- Create keycloak database (idempotent)
-- Uses \gexec to conditionally execute the CREATE DATABASE statement
SELECT 'CREATE DATABASE keycloak OWNER keycloak'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'keycloak')\gexec
