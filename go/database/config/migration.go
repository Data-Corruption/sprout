package config

import (
	"github.com/Data-Corruption/lmdb-go/lmdb"
)

type MigrationFunc func(txn *lmdb.Txn, dbi lmdb.DBI, schemas map[string]schema) error

var Migrations = map[string]MigrationFunc{
	"v0.0.1->v0.0.2": migrateV0_0_1toV0_0_2, // Example
}

// Example migration function
func migrateV0_0_1toV0_0_2(txn *lmdb.Txn, dbi lmdb.DBI, schemas map[string]schema) error {
	// Implement the migration logic here
	// Use old schema to read data and new schema to write updated data
	return nil
}
