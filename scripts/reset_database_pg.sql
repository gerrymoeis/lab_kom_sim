-- Reset Database Script (PostgreSQL / Neon DB)
-- Clean all data and reset sequences

-- Delete all data from tables (correct order for FK)
DELETE FROM maintenance_logs;
DELETE FROM software;
DELETE FROM logbook_entries;
DELETE FROM device_usages;
DELETE FROM device_loans;
DELETE FROM devices;
DELETE FROM device_types;
DELETE FROM pcs;
DELETE FROM users;

-- Reset sequences
ALTER SEQUENCE maintenance_logs_id_seq RESTART WITH 1;
ALTER SEQUENCE software_id_seq RESTART WITH 1;
ALTER SEQUENCE logbook_entries_id_seq RESTART WITH 1;
ALTER SEQUENCE device_usages_id_seq RESTART WITH 1;
ALTER SEQUENCE device_loans_id_seq RESTART WITH 1;
ALTER SEQUENCE devices_id_seq RESTART WITH 1;
ALTER SEQUENCE device_types_id_seq RESTART WITH 1;
ALTER SEQUENCE pcs_id_seq RESTART WITH 1;
ALTER SEQUENCE users_id_seq RESTART WITH 1;
