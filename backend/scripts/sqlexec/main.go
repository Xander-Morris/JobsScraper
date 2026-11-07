//go:build tools

// Command sqlexec runs one SQL statement, so gen-db.sh doesn't need psql installed.
package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: sqlexec <dsn> <statement>")
		os.Exit(2)
	}

	db, err := sql.Open("pgx", os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()

	if _, err := db.Exec(os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
