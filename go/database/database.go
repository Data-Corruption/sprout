// Package database provides functions to manage the LMDB wrapper for the application.
package database

import (
	"context"
	"errors"
	"path/filepath"
	"sprout/go/database/datapath"

	"github.com/Data-Corruption/lmdb-go/wrap"
)

/*
Database Layout:

Config - see config package for details.

Add other db info here.

*/

const (
	ConfigDBIName = "config"
	// Add more DBI names as needed, e.g., UserDBIName, SessionDBIName, etc. Also update the slice below to include them.
	// WARNING: If you add more DBIs you'll need to clean and reinitialize the database from scratch pretty sure.
)

type ctxKey struct{}

func IntoContext(ctx context.Context, db *wrap.DB) context.Context {
	return context.WithValue(ctx, ctxKey{}, db)
}

func FromContext(ctx context.Context) *wrap.DB {
	if db, ok := ctx.Value(ctxKey{}).(*wrap.DB); ok {
		return db
	}
	return nil
}

func New(ctx context.Context) (*wrap.DB, error) {
	path := datapath.FromContext(ctx)
	if path == "" {
		return nil, errors.New("nexus data path not set before database initialization")
	}
	db, _, err := wrap.New(filepath.Join(path, "db"),
		[]string{ConfigDBIName}, // If you add more DBIs, update this slice as well.
	)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
