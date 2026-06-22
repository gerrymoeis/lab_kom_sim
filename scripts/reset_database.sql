-- Reset Database Script
-- This script will clean all data and reset auto-increment counters

-- Delete all data from tables (in correct order to respect foreign keys)
DELETE FROM maintenance_logs;
DELETE FROM software;
DELETE FROM logbook_entries;
DELETE FROM devices;
DELETE FROM pcs;

-- Reset auto-increment counters
DELETE FROM sqlite_sequence WHERE name='maintenance_logs';
DELETE FROM sqlite_sequence WHERE name='software';
DELETE FROM sqlite_sequence WHERE name='logbook_entries';
DELETE FROM sqlite_sequence WHERE name='devices';
DELETE FROM sqlite_sequence WHERE name='pcs';

-- Verify reset
SELECT name, seq FROM sqlite_sequence;
