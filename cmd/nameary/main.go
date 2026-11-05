package main

import (
	"os"

	"github.com/yufeiminds/nameary/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
