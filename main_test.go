package main

import (
	"database/sql"
	"database/sql/driver"
	"io"
	"testing"
)

func init() {
	sql.Register("testmain", &successDrv{})
}

// successDrv is a minimal sql/driver that succeeds on Exec (no external libs).

type successDrv struct{}
type successConn struct{}
type successStmt struct{}
type successRows struct{}
type successResult struct{}

func (*successDrv) Open(_ string) (driver.Conn, error)            { return &successConn{}, nil }
func (*successConn) Prepare(_ string) (driver.Stmt, error)        { return &successStmt{}, nil }
func (*successConn) Close() error                                 { return nil }
func (*successConn) Begin() (driver.Tx, error)                    { return nil, nil }
func (*successStmt) Close() error                                 { return nil }
func (*successStmt) NumInput() int                                { return -1 }
func (*successStmt) Exec(_ []driver.Value) (driver.Result, error) { return successResult{}, nil }
func (*successStmt) Query(_ []driver.Value) (driver.Rows, error)  { return &successRows{}, nil }
func (successResult) LastInsertId() (int64, error)                { return 0, nil }
func (successResult) RowsAffected() (int64, error)                { return 0, nil }
func (*successRows) Columns() []string                            { return nil }
func (*successRows) Close() error                                 { return nil }
func (*successRows) Next(_ []driver.Value) error                  { return io.EOF }

func TestMigrateDB_Success(t *testing.T) {
	db, err := sql.Open("testmain", "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()
	// migrateDB executes 3 DDL statements — successDrv returns nil for all
	migrateDB(db)
}
