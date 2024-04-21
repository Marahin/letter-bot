//go:build tools
// +build tools

/*
This file is used for binary dependencies (such as code generators).
More on that here:
https://marcofranssen.nl/manage-go-tools-via-go-modules
https://www.liatrio.com/resources/blog/tools-go-pattern-modern-go-development
https://play-with-go.dev/tools-as-dependencies_go119_en/
*/
package main

import (
	_ "github.com/fzipp/gocyclo/cmd/gocyclo"
	_ "github.com/sqlc-dev/sqlc"
)
