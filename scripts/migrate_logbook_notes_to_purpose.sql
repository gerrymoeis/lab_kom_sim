-- Migration: Rename 'notes' column to 'purpose' in logbook_entries table
-- Date: 2026-05-06
-- Description: Change column name from 'notes' (keterangan) to 'purpose' (keperluan)

-- SQLite doesn't support RENAME COLUMN directly in older versions
-- So we need to recreate the table

-- Step 1: Create new table with correct column name
CREATE TABLE IF NOT EXISTS logbook_entries_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date DATE NOT NULL,
    student_name TEXT NOT NULL,
    nim TEXT NOT NULL,
    time_in TEXT NOT NULL,
    time_out TEXT,
    purpose TEXT,  -- Changed from 'notes' to 'purpose'
    source_file TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Step 2: Copy data from old table to new table
INSERT INTO logbook_entries_new (id, date, student_name, nim, time_in, time_out, purpose, source_file, created_at, updated_at)
SELECT id, date, student_name, nim, time_in, time_out, notes, source_file, created_at, updated_at
FROM logbook_entries;

-- Step 3: Drop old table
DROP TABLE logbook_entries;

-- Step 4: Rename new table to original name
ALTER TABLE logbook_entries_new RENAME TO logbook_entries;

-- Step 5: Recreate indexes
CREATE INDEX IF NOT EXISTS idx_logbook_date ON logbook_entries(date);
CREATE INDEX IF NOT EXISTS idx_logbook_nim ON logbook_entries(nim);

-- Verification query (optional - comment out in production)
-- SELECT COUNT(*) as total_entries FROM logbook_entries;
