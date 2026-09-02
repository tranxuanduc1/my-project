SELECT 'CREATE DATABASE iam'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'iam')\gexec

SELECT 'CREATE DATABASE orders'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'orders')\gexec

SELECT 'CREATE DATABASE payments'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'payments')\gexec
