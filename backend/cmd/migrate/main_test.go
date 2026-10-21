package main

import (
	"errors"
	"testing"
)

func stub(t *testing.T, ensureErr, migrateErr error) *int {
	t.Helper()
	prevExit, prevEnsure, prevCreate := exit, ensure, createTables
	t.Cleanup(func() { exit, ensure, createTables = prevExit, prevEnsure, prevCreate })

	code := -1
	exit = func(c int) { code = c }
	ensure = func(...string) error { return ensureErr }
	createTables = func() error { return migrateErr }
	return &code
}

func TestMigrate(t *testing.T) {
	for name, tc := range map[string]struct {
		ensureErr, migrateErr error
		want                  int
	}{
		"up to date":        {want: 0},
		"bad config":        {ensureErr: errors.New("missing env"), want: 1},
		"migration failure": {migrateErr: errors.New("bad sql"), want: 1},
	} {
		code := stub(t, tc.ensureErr, tc.migrateErr)
		main()
		if *code != tc.want {
			t.Errorf("%s: exit %d, want %d", name, *code, tc.want)
		}
	}
}
