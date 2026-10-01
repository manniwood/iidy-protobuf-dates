-- TO RUN, DO THIS:
-- $ psql -X -U postgres -d postgres -f setup.sql 
create user iidy password 'iidy';
create database iidy with owner iidy;
