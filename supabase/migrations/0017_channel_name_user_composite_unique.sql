-- Change channel name uniqueness from global to per-user
-- Drop the old global unique constraint and add composite (name, user_id) unique constraint

ALTER TABLE public.channels DROP CONSTRAINT IF EXISTS channels_name_unique;
ALTER TABLE public.channels DROP CONSTRAINT IF EXISTS channels_name_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_channels_name_user ON public.channels (name, user_id);
