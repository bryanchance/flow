create table accounts (id serial primary key, account jsonb);
create table namespaces (id serial primary key, namespace jsonb);
create table workflows (id serial primary key, workflow jsonb);
create table servicetokens (id serial primary key, servicetoken jsonb);
create table apitokens (id serial primary key, apitoken jsonb);
create table queue (id serial primary key, workflow jsonb);

create table authenticator (id serial primary key, key varchar, value bytea, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
