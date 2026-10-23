package migrations

import _ "embed"

//go:embed 001_initial.up.sql
var Up string
