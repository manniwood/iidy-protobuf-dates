package data

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is anything that can run Query() from jackc's pgx library.
// Usefully, all of these pgx structs/interfaces have the same signature for Query():
//
//	pgx.Conn
//	pgx.Tx
//	pgxpool.Conn
//	pgxpool.Pool
//
// So if a function implements Querier, then that function can take, as an argument,
// any of the above, meaning that function will automatically work with pgx connections,
// connection pools, transactions, etc.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Execer is anything that can run Exec() from jackc's pgx library.
// Usefully, all of these pgx structs/interfaces have the same signature for Exec():
//
//	pgx.Conn
//	pgx.Tx
//	pgxpool.Conn
//	pgxpool.Pool
//
// So if a function implements Execer, then that function can take, as an argument,
// any of the above, meaning that function will automatically work with pgx connections,
// connection pools, transactions, etc.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// CopyFromer is anything that can run a Postgres "copy from" command.
// Usefully, all these pgx structs/interfaces have the same signature
// for CopyFrom:
//
//	pgx.Conn
//	pgx.Tx
//	pgxpool.Conn
//	pgxpool.Pool
//
// So if a function implements CopyFromer, then that function can take, as an argument,
// any of the above, meaning that function will automatically work with pgx connections,
// connection pools, transactions, etc.
type CopyFromer interface {
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}

// QuerierExecer is anything that can run Query() and Exec() from jackc's pgx library.
// Usefully, all of these pgx structs/interfaces have the same signature for Query()
// and Exec():
//
//	pgx.Conn
//	pgx.Tx
//	pgxpool.Conn
//	pgxpool.Pool
//
// So if a function implements QuerierExecer, then that function can take, as an argument,
// any of the above, meaning that function will automatically work with pgx connections,
// connection pools, transactions, etc.
type QuerierExecer interface {
	Querier
	Execer
}

// Closer is used for running `defer xxx.Close()` on the pooler later
type Closer interface {
	Close()
}
