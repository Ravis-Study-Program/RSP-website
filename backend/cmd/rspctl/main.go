package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "seed" && os.Args[1] != "bootstrap-admin") {
		fmt.Fprintln(os.Stderr, "usage: rspctl seed|bootstrap-admin")
		os.Exit(2)
	}
	fmt.Printf("%s requires DATABASE_URL\n", os.Args[1])
}
