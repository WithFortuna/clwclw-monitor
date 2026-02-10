ALTER TABLE public.channels
ADD CONSTRAINT channels_name_unique UNIQUE (name);
