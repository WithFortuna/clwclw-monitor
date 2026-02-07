-- 0004_add_chains_and_chain_to_tasks.sql

-- Create the chains table
CREATE TABLE chains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'queued',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add chain_id and sequence columns to the tasks table
ALTER TABLE tasks
ADD COLUMN chain_id UUID REFERENCES chains(id) ON DELETE SET NULL,
ADD COLUMN sequence INTEGER;

-- Create an index on chain_id for faster lookups
CREATE INDEX idx_tasks_chain_id ON tasks(chain_id);

-- Optionally, add a unique constraint for sequence within a chain if needed
-- ALTER TABLE tasks
-- ADD CONSTRAINT unique_chain_sequence UNIQUE (chain_id, sequence);

-- Update updated_at column on changes for chains table
CREATE TRIGGER handle_updated_at_chains BEFORE UPDATE ON chains
FOR EACH ROW EXECUTE FUNCTION moddatetime();

-- Update updated_at column on changes for tasks table
-- Assuming moddatetime function already exists from previous migrations or is handled by Supabase
-- If not, ensure it's created, e.g.:
-- CREATE OR REPLACE FUNCTION moddatetime() RETURNS TRIGGER AS $$
-- BEGIN
--     NEW.updated_at = NOW();
--     RETURN NEW;
-- END;
-- $$ LANGUAGE plpgsql;
--
-- CREATE TRIGGER handle_updated_at_tasks BEFORE UPDATE ON tasks
-- FOR EACH ROW EXECUTE FUNCTION moddatetime();
