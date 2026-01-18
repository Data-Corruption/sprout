package config

import (
	"sprout/internal/platform/database"
	"sprout/internal/types"

	"github.com/Data-Corruption/lmdb-go/wrap"
)

// View retrieves a copy of the current configuration from the database.
//
// WARNING: Starts a transaction. Avoid nesting transactions (will deadlock).
func View(db *wrap.DB) (*types.Configuration, error) {
	return database.View[types.Configuration](db, *database.ConfigDBI, []byte(database.ConfigDataKey))
}

// Update updates the configuration in the database using the provided update function.
//
// WARNING: Starts a transaction. Avoid nesting transactions (will deadlock).
func Update(db *wrap.DB, updateFunc func(cfg *types.Configuration) error) error {
	return database.Update(db, *database.ConfigDBI, []byte(database.ConfigDataKey), updateFunc)
}
