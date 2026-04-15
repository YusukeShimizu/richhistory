package main

import (
	"os"
	"path/filepath"

	"github.com/YusukeShimizu/richhistory/internal/app"
)

func main() {
	os.Exit(app.RunAs(filepath.Base(os.Args[0]), os.Args[1:], os.Stdout, os.Stderr))
}
