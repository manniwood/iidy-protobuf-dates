
-- IMPORTANT:
-- 1) Use timestamp without time zone so that dates are never "translated"
-- to other time zones.
-- 2) Use localtimestamp, which provides the time in the *server's* time zone,
-- which *should* be set to UTC.

alter table iidy.lists add column created_at
  timestamp without time zone not null default localtimestamp;

alter table iidy.lists add column updated_at
  timestamp without time zone not null default localtimestamp;

