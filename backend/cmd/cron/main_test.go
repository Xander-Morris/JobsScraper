package main

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

func stub(t *testing.T, ensureErr error) (ran *string, code *int) {
	t.Helper()
	prevExit, prevEnsure, prevJobs := exit, ensure, jobs
	t.Cleanup(func() { exit, ensure, jobs = prevExit, prevEnsure, prevJobs })

	var job string
	c := -1
	exit = func(n int) { c = n }
	ensure = func(...string) error { return ensureErr }
	jobs = map[string]func(context.Context){"scrape": func(context.Context) { job = "scrape" }}
	return &job, &c
}

func TestRun(t *testing.T) {
	ran, _ := stub(t, nil)

	if code := run(t.Context(), []string{"-job=scrape"}, io.Discard); code != 0 || *ran != "scrape" {
		t.Errorf("run = %d, ran %q", code, *ran)
	}
	if code := run(t.Context(), []string{"-bogus"}, io.Discard); code != 2 {
		t.Errorf("bad flag = %d, want 2", code)
	}
	if code := run(t.Context(), []string{"-job=nope"}, io.Discard); code != 2 {
		t.Errorf("unknown job = %d, want 2", code)
	}
}

func TestRunBadConfig(t *testing.T) {
	stub(t, errors.New("missing env"))

	if code := run(t.Context(), []string{"-job=scrape"}, io.Discard); code != 1 {
		t.Errorf("run = %d, want 1", code)
	}
}

func TestMainExitsWithRunCode(t *testing.T) {
	_, code := stub(t, nil)
	prevArgs := os.Args
	os.Args = []string{"cron", "-job=nope"}
	t.Cleanup(func() { os.Args = prevArgs })

	main()

	if *code != 2 {
		t.Errorf("exit %d, want 2", *code)
	}
}
