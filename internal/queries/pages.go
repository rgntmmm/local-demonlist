package queries

import (
	"embed"
	"fmt"
)

//go:embed pages/*.sql
var pagesFS embed.FS

func Pages(name string) string {
	b, err := pagesFS.ReadFile("pages/" + name + ".sql")
	if err != nil {
		panic(fmt.Sprintf("query %q not found: %v", name, err))
	}
	return string(b)
}
