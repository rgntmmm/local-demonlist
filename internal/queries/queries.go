package queries

import (
	"embed"
	"fmt"
)

//go:embed levels/*.sql
var levelsFS embed.FS

func Levels(name string) string {
	b, err := levelsFS.ReadFile("levels/" + name + ".sql")
	if err != nil {
		panic(fmt.Sprintf("query %q not found: %v", name, err))
	}
	return string(b)
}