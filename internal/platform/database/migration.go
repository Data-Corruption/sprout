package database

import (
	"fmt"
	"sprout/pkg/migrator"
	"time"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/lmdb-go/wrap"
	"github.com/Data-Corruption/stdx/xlog"
)

func Migrate(db *wrap.DB, logger *xlog.Logger) error {
	m := migrator.New()

	// Add steps here. Order matters!

	m.Add("v1", "Initial Schema", func(txn *lmdb.Txn) error {
		// Get cfg DBI
		cfgDBI, ok := db.GetDBis()[ConfigDBIName]
		if !ok {
			return fmt.Errorf("config DBI not found")
		}

		// Create Config with default values
		cfg := Configuration{
			LogLevel:            "WARN",
			Port:                8080,
			Host:                "localhost",
			UpdateNotifications: true,
			LastUpdateCheck:     time.Now(),
		}

		// Store config
		if err := TxnMarshalAndPut(txn, cfgDBI, []byte(ConfigDataKey), cfg); err != nil {
			return fmt.Errorf("failed to store initial config: %w", err)
		}

		return nil
	})

	/* Example version bump
	migrator.Add("v2", "Add Thing to Thing", func(txn *lmdb.Txn) error {
		// do v2 stuff
		return nil
	})
	*/

	return db.Update(func(txn *lmdb.Txn) error {
		// Get current version
		cfgDBI, ok := db.GetDBis()[ConfigDBIName]
		if !ok {
			return fmt.Errorf("config DBI not found")
		}
		currentVer := ""
		if err := TxnGetAndUnmarshal(txn, cfgDBI, []byte(ConfigVersionKey), &currentVer); err != nil {
			if !lmdb.IsNotFound(err) {
				return fmt.Errorf("failed to get config version: %w", err)
			}
			currentVer = ""
		}

		// Run migrations
		newVer, err := m.Run(txn, currentVer, logger)
		if err != nil {
			return err
		}

		// Update version in DB
		if err := TxnMarshalAndPut(txn, cfgDBI, []byte(ConfigVersionKey), newVer); err != nil {
			return fmt.Errorf("failed to update config version: %w", err)
		}

		logger.Infof("Migrated from %q to %q\n", currentVer, newVer)
		return nil
	})
}
