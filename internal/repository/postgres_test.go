package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"

	"github.com/hack-fiap233/users/internal/service"
	"github.com/lib/pq"
)

// --- fake SQL driver (stdlib only, no external mock libs) ---

func init() {
	sql.Register("noopdb", &noopDrv{})
}

type noopDrv struct{}
type noopDrvConn struct{}
type noopDrvStmt struct{}
type noopDrvRows struct{}

func (*noopDrv) Open(_ string) (driver.Conn, error)          { return &noopDrvConn{}, nil }
func (*noopDrvConn) Prepare(_ string) (driver.Stmt, error)   { return &noopDrvStmt{}, nil }
func (*noopDrvConn) Close() error                            { return nil }
func (*noopDrvConn) Begin() (driver.Tx, error)               { return nil, errors.New("not supported") }
func (*noopDrvStmt) Close() error                            { return nil }
func (*noopDrvStmt) NumInput() int                           { return -1 }
func (*noopDrvStmt) Exec(_ []driver.Value) (driver.Result, error) {
	return nil, errors.New("not implemented")
}
func (*noopDrvStmt) Query(_ []driver.Value) (driver.Rows, error) { return &noopDrvRows{}, nil }
func (*noopDrvRows) Columns() []string                           { return nil }
func (*noopDrvRows) Close() error                                { return nil }
func (*noopDrvRows) Next(_ []driver.Value) error                 { return io.EOF }

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("noopdb", "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	return db
}

// --- fake row scanner ---

type fakeRow struct {
	vals []any
	err  error
}

func (f *fakeRow) Scan(dest ...any) error {
	if f.err != nil {
		return f.err
	}
	for i, d := range dest {
		if i >= len(f.vals) {
			break
		}
		switch v := d.(type) {
		case *int:
			*v = f.vals[i].(int)
		case *string:
			*v = f.vals[i].(string)
		}
	}
	return nil
}

// --- fake rows ---

type fakeRows struct {
	data    [][]any
	index   int
	scanErr error
	rowsErr error
}

func (f *fakeRows) Next() bool {
	f.index++
	return f.index <= len(f.data)
}

func (f *fakeRows) Scan(dest ...any) error {
	if f.scanErr != nil {
		return f.scanErr
	}
	row := f.data[f.index-1]
	for i, d := range dest {
		if i >= len(row) {
			break
		}
		switch v := d.(type) {
		case *int:
			*v = row[i].(int)
		case *string:
			*v = row[i].(string)
		}
	}
	return nil
}

func (f *fakeRows) Close() error { return nil }
func (f *fakeRows) Err() error   { return f.rowsErr }

// --- mock db ---

type mockDB struct {
	queryRowFunc func(ctx context.Context, query string, args ...any) rowScanner
	queryFunc    func(ctx context.Context, query string, args ...any) (sqlRows, error)
	pingFunc     func(ctx context.Context) error
}

func (m *mockDB) QueryRowContext(ctx context.Context, query string, args ...any) rowScanner {
	return m.queryRowFunc(ctx, query, args...)
}
func (m *mockDB) QueryContext(ctx context.Context, query string, args ...any) (sqlRows, error) {
	return m.queryFunc(ctx, query, args...)
}
func (m *mockDB) PingContext(ctx context.Context) error {
	return m.pingFunc(ctx)
}

func newRepo(db dbQuerier) *postgresRepository {
	return &postgresRepository{db: db}
}

// --- Create ---

func TestCreate_Success(t *testing.T) {
	db := &mockDB{queryRowFunc: func(_ context.Context, _ string, _ ...any) rowScanner {
		return &fakeRow{vals: []any{1, "Alice", "alice@example.com"}}
	}}
	u, err := newRepo(db).Create(context.Background(), "Alice", "alice@example.com", "hash")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if u.ID != 1 || u.Name != "Alice" || u.Email != "alice@example.com" {
		t.Errorf("unexpected user: %+v", u)
	}
}

func TestCreate_DBError(t *testing.T) {
	db := &mockDB{queryRowFunc: func(_ context.Context, _ string, _ ...any) rowScanner {
		return &fakeRow{err: errors.New("db error")}
	}}
	_, err := newRepo(db).Create(context.Background(), "Alice", "alice@example.com", "hash")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreate_DuplicateEmail(t *testing.T) {
	db := &mockDB{queryRowFunc: func(_ context.Context, _ string, _ ...any) rowScanner {
		return &fakeRow{err: &pq.Error{Code: "23505"}}
	}}
	_, err := newRepo(db).Create(context.Background(), "Alice", "alice@example.com", "hash")
	if !errors.Is(err, service.ErrDuplicateEmail) {
		t.Errorf("expected ErrDuplicateEmail, got %v", err)
	}
}

// --- FindByEmail ---

func TestFindByEmail_Success(t *testing.T) {
	db := &mockDB{queryRowFunc: func(_ context.Context, _ string, _ ...any) rowScanner {
		return &fakeRow{vals: []any{1, "Alice", "alice@example.com", "hashed_pw"}}
	}}
	u, hash, err := newRepo(db).FindByEmail(context.Background(), "alice@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if u.ID != 1 || u.Name != "Alice" || u.Email != "alice@example.com" {
		t.Errorf("unexpected user: %+v", u)
	}
	if hash != "hashed_pw" {
		t.Errorf("expected hash 'hashed_pw', got %q", hash)
	}
}

func TestFindByEmail_NotFound(t *testing.T) {
	db := &mockDB{queryRowFunc: func(_ context.Context, _ string, _ ...any) rowScanner {
		return &fakeRow{err: sql.ErrNoRows}
	}}
	_, _, err := newRepo(db).FindByEmail(context.Background(), "nobody@example.com")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestFindByEmail_DBError(t *testing.T) {
	db := &mockDB{queryRowFunc: func(_ context.Context, _ string, _ ...any) rowScanner {
		return &fakeRow{err: errors.New("connection lost")}
	}}
	_, _, err := newRepo(db).FindByEmail(context.Background(), "alice@example.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- List ---

func TestList_Success(t *testing.T) {
	db := &mockDB{queryFunc: func(_ context.Context, _ string, _ ...any) (sqlRows, error) {
		return &fakeRows{data: [][]any{
			{1, "Alice", "alice@example.com"},
			{2, "Bob", "bob@example.com"},
		}}, nil
	}}
	users, err := newRepo(db).List(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
	if users[0].Name != "Alice" || users[1].Name != "Bob" {
		t.Errorf("unexpected users: %+v", users)
	}
}

func TestList_Empty(t *testing.T) {
	db := &mockDB{queryFunc: func(_ context.Context, _ string, _ ...any) (sqlRows, error) {
		return &fakeRows{data: [][]any{}}, nil
	}}
	users, err := newRepo(db).List(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected empty slice, got %d", len(users))
	}
}

func TestList_QueryError(t *testing.T) {
	db := &mockDB{queryFunc: func(_ context.Context, _ string, _ ...any) (sqlRows, error) {
		return nil, errors.New("db error")
	}}
	_, err := newRepo(db).List(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestList_ScanError(t *testing.T) {
	db := &mockDB{queryFunc: func(_ context.Context, _ string, _ ...any) (sqlRows, error) {
		return &fakeRows{
			data:    [][]any{{1, "Alice", "alice@example.com"}},
			scanErr: errors.New("scan error"),
		}, nil
	}}
	_, err := newRepo(db).List(context.Background())
	if err == nil {
		t.Fatal("expected error from scan")
	}
}

func TestList_RowsErr(t *testing.T) {
	db := &mockDB{queryFunc: func(_ context.Context, _ string, _ ...any) (sqlRows, error) {
		return &fakeRows{
			data:    [][]any{},
			rowsErr: errors.New("rows iteration error"),
		}, nil
	}}
	_, err := newRepo(db).List(context.Background())
	if err == nil {
		t.Fatal("expected error from rows.Err()")
	}
}

// --- Ping ---

func TestPing_Success(t *testing.T) {
	db := &mockDB{pingFunc: func(_ context.Context) error { return nil }}
	if err := newRepo(db).Ping(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPing_Error(t *testing.T) {
	db := &mockDB{pingFunc: func(_ context.Context) error { return errors.New("connection refused") }}
	if err := newRepo(db).Ping(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

// --- Builder ---

func TestNew_ReturnsBuilder(t *testing.T) {
	if New() == nil {
		t.Fatal("expected non-nil builder")
	}
}

func TestBuild_WithoutDB(t *testing.T) {
	if New().Build() == nil {
		t.Fatal("expected non-nil repository")
	}
}

func TestWithDB_Build(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	if New().WithDB(db).Build() == nil {
		t.Fatal("expected non-nil repository")
	}
}

// --- sqlDBAdapter ---

func TestSqlDBAdapter_PingContext(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	adapter := &sqlDBAdapter{db: db}
	// noopDrvConn doesn't implement driver.Pinger, so sql.DB just verifies
	// the connection is openable — should return nil
	if err := adapter.PingContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSqlDBAdapter_QueryRowContext(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	adapter := &sqlDBAdapter{db: db}
	row := adapter.QueryRowContext(context.Background(), "SELECT 1")
	if row == nil {
		t.Fatal("expected non-nil row scanner")
	}
}

func TestSqlDBAdapter_QueryContext(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	adapter := &sqlDBAdapter{db: db}
	rows, err := adapter.QueryContext(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows.Close()
}
