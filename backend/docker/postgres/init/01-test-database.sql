-- Runs once, when the local Compose database volume is first created.
-- Go integration tests use this separate database (TEST_DATABASE_URL) so
-- they never touch development data.
CREATE DATABASE dakota_test;
