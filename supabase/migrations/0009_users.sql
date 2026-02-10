-- Users table for authentication
create table if not exists public.users (
    id uuid primary key default gen_random_uuid(),
    username text not null,
    password_hash text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create unique index if not exists users_username_lower_idx on public.users (lower(username));
