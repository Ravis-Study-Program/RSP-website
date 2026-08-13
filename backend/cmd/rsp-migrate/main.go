package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: rsp-migrate legacy dry-run|apply|verify|rollback")
		os.Exit(2)
	}
	fmt.Fprintln(os.Stderr, "migration database is not configured")
}
