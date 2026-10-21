// Package faultdb wraps the pgx driver so tests can fail one chosen database
// operation and reach the error branches a healthy Postgres never takes.
// Test-only: nothing outside _test.go files imports it.
package faultdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
)

// ErrInjected is what a failed operation returns.
var ErrInjected = errors.New("faultdb: injected fault")

// Mode picks how the armed operation misbehaves.
type Mode int

const (
	// Fail makes the Nth operation of any kind return ErrInjected.
	Fail Mode = iota
	// Corrupt makes the Nth fetched row carry values no Scan accepts.
	Corrupt
	// Garbage breaks the Nth JSON value fetched.
	Garbage
)

func (m Mode) String() string {
	return [...]string{"fail", "corrupt", "garbage"}[m]
}

// Injector counts operations and misbehaves on the armed one.
type Injector struct {
	mu     sync.Mutex
	armed  bool
	failAt int
	count  int
	mode   Mode
	fired  bool
	only   string

	beforeQuery string
	beforeFn    func()
}

// Before runs fn once, just before the next query containing substr, so a
// test can change the database mid-call (e.g. to stage a race).
func (i *Injector) Before(substr string, fn func()) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.beforeQuery, i.beforeFn = substr, fn
}

func (i *Injector) runBefore(query string) {
	i.mu.Lock()
	fn := i.beforeFn
	if fn == nil || !strings.Contains(query, i.beforeQuery) {
		i.mu.Unlock()
		return
	}
	i.beforeFn = nil
	i.mu.Unlock()

	fn()
}

// Arm misbehaves on the nth operation counted from now.
func (i *Injector) Arm(n int, mode Mode) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.armed, i.failAt, i.count, i.mode, i.fired = true, n, 0, mode, false
}

// Disarm stops injecting and reports whether the armed operation was reached.
func (i *Injector) Disarm() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.armed = false
	return i.fired
}

// Only limits counting to operations whose SQL contains substr, so calls
// that run queries concurrently can be swept deterministically. The returned
// func lifts the limit.
func (i *Injector) Only(substr string) func() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.only = substr
	return func() { i.Only("") }
}

// hit counts an operation of the given mode and reports whether it is the armed one.
func (i *Injector) hit(mode Mode, query string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()

	if !i.armed || i.fired || i.mode != mode || !strings.Contains(query, i.only) {
		return false
	}

	i.count++
	i.fired = i.count == i.failAt
	return i.fired
}

func (i *Injector) check(query string) error {
	if i.hit(Fail, query) {
		return ErrInjected
	}
	return nil
}

// Open returns a pool on connString whose operations inj can fail.
func Open(connString string) (*sql.DB, *Injector, error) {
	base, err := stdlib.GetDefaultDriver().(driver.DriverContext).OpenConnector(connString)
	if err != nil {
		return nil, nil, err
	}

	inj := &Injector{}
	return sql.OpenDB(&connector{base: base, inj: inj}), inj, nil
}

// Sweep reruns call, failing each of its operations in turn (then corrupting
// each row it fetches, then breaking each JSON value), and reports any run
// that swallowed the fault. setup,
// if set, runs unarmed before every attempt to rebuild state.
func Sweep(t *testing.T, inj *Injector, setup func(), call func() error) {
	t.Helper()

	var err error
	SweepEach(t, inj, setup, func() { err = call() }, func(t *testing.T) {
		if err == nil {
			t.Errorf("fault ignored")
		}
	})
}

// SweepEach is Sweep for calls meant to swallow faults: check runs after
// every attempt whose fault was reached, and makes its own assertions.
func SweepEach(t *testing.T, inj *Injector, setup func(), call func(), check func(t *testing.T)) {
	t.Helper()

	for _, mode := range []Mode{Fail, Corrupt, Garbage} {
		for n := 1; ; n++ {
			if setup != nil {
				setup()
			}

			inj.Arm(n, mode)
			call()
			if !inj.Disarm() {
				break
			}

			t.Run(fmt.Sprintf("%s-%d", mode, n), check)
		}
	}
}

type connector struct {
	base driver.Connector
	inj  *Injector
}

func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	inner, err := c.base.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &conn{inner: inner, inj: c.inj}, nil
}

func (c *connector) Driver() driver.Driver { return c.base.Driver() }

// pgx's stdlib conn implements every optional interface used here.
type innerConn interface {
	driver.Conn
	driver.ConnBeginTx
	driver.ConnPrepareContext
	driver.ExecerContext
	driver.QueryerContext
	driver.Pinger
	driver.NamedValueChecker
	driver.SessionResetter
}

type conn struct {
	inner driver.Conn
	inj   *Injector
}

func (c *conn) full() innerConn { return c.inner.(innerConn) }

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if err := c.inj.check(query); err != nil {
		return nil, err
	}
	return c.full().PrepareContext(ctx, query)
}

func (c *conn) Close() error { return c.inner.Close() }

func (c *conn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if err := c.inj.check(""); err != nil {
		return nil, err
	}
	tx, err := c.full().BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &txn{inner: tx, inj: c.inj}, nil
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.inj.runBefore(query)
	if err := c.inj.check(query); err != nil {
		return nil, err
	}
	res, err := c.full().ExecContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	return &result{inner: res, inj: c.inj, query: query}, nil
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.inj.runBefore(query)
	if err := c.inj.check(query); err != nil {
		return nil, err
	}
	r, err := c.full().QueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	return &rows{inner: r, inj: c.inj, query: query}, nil
}

func (c *conn) Ping(ctx context.Context) error { return c.full().Ping(ctx) }

func (c *conn) CheckNamedValue(v *driver.NamedValue) error { return c.full().CheckNamedValue(v) }

func (c *conn) ResetSession(ctx context.Context) error { return c.full().ResetSession(ctx) }

type txn struct {
	inner driver.Tx
	inj   *Injector
}

func (t *txn) Commit() error {
	if err := t.inj.check(""); err != nil {
		_ = t.inner.Rollback()
		return err
	}
	return t.inner.Commit()
}

func (t *txn) Rollback() error { return t.inner.Rollback() }

type result struct {
	inner driver.Result
	inj   *Injector
	query string
}

func (r *result) LastInsertId() (int64, error) { return r.inner.LastInsertId() }

func (r *result) RowsAffected() (int64, error) {
	if err := r.inj.check(r.query); err != nil {
		return 0, err
	}
	return r.inner.RowsAffected()
}

type rows struct {
	inner driver.Rows
	inj   *Injector
	query string
}

func (r *rows) Columns() []string { return r.inner.Columns() }

func (r *rows) Close() error { return r.inner.Close() }

func (r *rows) Next(dest []driver.Value) error {
	if r.inj.hit(Fail, r.query) {
		return ErrInjected
	}

	if err := r.inner.Next(dest); err != nil {
		return err
	}

	switch {
	case r.inj.hit(Corrupt, r.query):
		for i := range dest {
			dest[i] = unscannable{}
		}
	default:
		for i, v := range dest {
			if isJSON(v) && r.inj.hit(Garbage, r.query) {
				dest[i] = "{"
			}
		}
	}
	return nil
}

// unscannable is a value database/sql cannot convert into any Go type.
type unscannable struct{}

func isJSON(v driver.Value) bool {
	var text string
	switch v := v.(type) {
	case string:
		text = v
	case []byte:
		text = string(v)
	default:
		return false
	}
	return strings.HasPrefix(text, "[") || strings.HasPrefix(text, "{")
}
