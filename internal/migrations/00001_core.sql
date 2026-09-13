-- +goose Up
create type beverage_type as enum ('wine','mead','cider');
create type batch_status as enum ('planned','fermenting','conditioning','bottled','archived');
create type event_type as enum ('measurement','addition','racking','stabilize','sweeten','bottling','note');
create type event_status as enum ('planned','done','skipped');
create type anchor_type as enum ('start','previous');
create type stage_tag as enum ('primary','secondary','stabilize','sweeten','bottling');
create type vessel_kind as enum ('carboy','bucket','barrel','tank','keg','other');
create type ingredient_category as enum ('fermentable','fruit','yeast','nutrient','chemical','other');
create type user_role as enum ('admin','member');

create table users (
  id uuid primary key default gen_random_uuid(),
  email text not null unique,
  name text not null,
  password_hash text not null,
  role user_role not null default 'member',
  created_at timestamptz not null default now()
);

create table sessions (
  id text primary key,
  user_id uuid not null references users(id) on delete cascade,
  expires_at timestamptz not null,
  created_at timestamptz not null default now()
);
create index sessions_user_idx on sessions(user_id);

create table invites (
  id uuid primary key default gen_random_uuid(),
  token text not null unique,
  email text,
  created_by uuid not null references users(id) on delete cascade,
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  used_at timestamptz,
  used_by uuid references users(id) on delete set null,
  revoked_at timestamptz
);

create table vessels (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  kind vessel_kind not null,
  capacity_l real not null,
  notes text,
  created_at timestamptz not null default now()
);

create table ingredients (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  category ingredient_category not null,
  unit text not null
);

create table inventory_items (
  id uuid primary key default gen_random_uuid(),
  ingredient_id uuid not null references ingredients(id),
  quantity real not null default 0,
  cost_per_unit real,
  updated_at timestamptz not null default now()
);
create unique index inventory_items_ingredient_idx on inventory_items(ingredient_id);

create table recipes (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  beverage beverage_type not null,
  style text,
  description text,
  base_volume_l real not null,
  target_og real,
  target_fg real,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table recipe_ingredients (
  id uuid primary key default gen_random_uuid(),
  recipe_id uuid not null references recipes(id) on delete cascade,
  ingredient_id uuid not null references ingredients(id),
  qty_per_base real not null,
  stage stage_tag not null,
  sort_index integer not null
);

create table recipe_steps (
  id uuid primary key default gen_random_uuid(),
  recipe_id uuid not null references recipes(id) on delete cascade,
  sort_index integer not null,
  type event_type not null,
  anchor anchor_type not null,
  offset_days integer not null,
  label text not null,
  data jsonb not null default '{}'
);

create table batches (
  id uuid primary key default gen_random_uuid(),
  recipe_id uuid references recipes(id),
  name text not null,
  beverage beverage_type not null,
  start_date date not null,
  volume_l real not null,
  status batch_status not null default 'fermenting',
  current_vessel_id uuid references vessels(id),
  created_at timestamptz not null default now()
);

create table batch_events (
  id uuid primary key default gen_random_uuid(),
  batch_id uuid not null references batches(id) on delete cascade,
  type event_type not null,
  status event_status not null default 'planned',
  sort_index integer not null,
  anchor anchor_type,
  offset_days integer,
  scheduled_date date,
  completed_date date,
  vessel_from_id uuid references vessels(id),
  vessel_to_id uuid references vessels(id),
  data jsonb not null default '{}',
  note text,
  created_by uuid references users(id) on delete set null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index batch_events_batch_sort_idx on batch_events(batch_id, sort_index);
create index batch_events_due_idx on batch_events(status, scheduled_date);

create table bottle_lots (
  id uuid primary key default gen_random_uuid(),
  batch_id uuid not null references batches(id),
  bottling_event_id uuid not null references batch_events(id),
  count integer not null,
  remaining integer not null,
  size_ml integer not null,
  drink_from date,
  drink_to date,
  location text,
  created_at timestamptz not null default now()
);

create table tasting_notes (
  id uuid primary key default gen_random_uuid(),
  lot_id uuid not null references bottle_lots(id) on delete cascade,
  date date not null,
  rating integer,
  note text,
  created_by uuid references users(id) on delete set null
);

-- +goose Down
drop table tasting_notes, bottle_lots, batch_events, batches, recipe_steps,
  recipe_ingredients, recipes, inventory_items, ingredients, vessels,
  invites, sessions, users;
drop type user_role, ingredient_category, vessel_kind, stage_tag, anchor_type,
  event_status, event_type, batch_status, beverage_type;
