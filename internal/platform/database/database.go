// Package database provides functions to manage the LMDB wrapper for the application.
package database

import (
	"github.com/Data-Corruption/lmdb-go/wrap"
	"github.com/Data-Corruption/stdx/xlog"
)

/*
Database Layout:

Config
    "version" -> version string of database schema (not app version)
	"data" -> marshaled config struct
Other DBIs
    "<name>" -> <data>

*/

const (
	ConfigVersionKey = "version"
	ConfigDataKey    = "data"

	// DBI Names
	ConfigDBIName = "config"
	// Add more DBI names as needed, e.g., UserDBIName, SessionDBIName, etc. Also update the slice below to include them.
	// My lmdb wrapper hard codes the max number of named dbis to 128.
)

// Slice for easy initialization. As stated above, if you add more DBIs you'll need to update this slice as well.
var DBINameList = []string{ConfigDBIName}

func New(directory string, logger *xlog.Logger) (*wrap.DB, error) {
	// Initialize LMDB with the specified DBIs
	db, srClosed, err := wrap.New(directory, DBINameList)
	if err != nil {
		if db != nil {
			db.Close()
		}
		return nil, err
	}
	logger.Infof("LMDB initialized at %s", directory)
	if srClosed > 0 {
		logger.Warnf("LMDB had %d stale readers which were closed", srClosed)
	}

	// Perform migrations if needed
	if err := Migrate(db, logger); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
