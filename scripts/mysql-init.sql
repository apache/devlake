-- Create test database for E2E tests
CREATE DATABASE IF NOT EXISTS lake_test;

-- Grant permissions to merico user
GRANT ALL PRIVILEGES ON lake_test.* TO 'merico'@'%';
FLUSH PRIVILEGES;
