create schema iidy;

create table iidy.lists (
	list     text    not null,
	item     text    not null,
	attempts integer not null default 0,
	constraint list_pk primary key (list, item));
