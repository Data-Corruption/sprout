package database

import (
	"encoding/json"
	"fmt"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/lmdb-go/wrap"
)

// TxnMarshalAndPut marshals the provided value and stores it in the database under the given key.
func TxnMarshalAndPut(txn *lmdb.Txn, dbi lmdb.DBI, key []byte, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := txn.Put(dbi, key, data, 0); err != nil {
		return err
	}
	return nil
}

// TxnGetAndUnmarshal retrieves a value from the database and unmarshals it into the provided value pointer.
// lmdb.IsNotFound(err) will be true if the key was not found in the database.
func TxnGetAndUnmarshal(txn *lmdb.Txn, dbi lmdb.DBI, key []byte, value any) error {
	buf, err := txn.Get(dbi, key)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(buf, value); err != nil {
		return err
	}
	return nil
}

// ViewConfig retrieves a copy of the current configuration from the database.
func ViewConfig(db *wrap.DB) (*Configuration, error) {
	var cfg Configuration

	data, err := db.Read(ConfigDBIName, []byte(ConfigDataKey))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// UpdateConfig updates the configuration in the database using the provided update function.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func UpdateConfig(db *wrap.DB, updateFunc func(cfg *Configuration) error) error {
	return db.Update(func(txn *lmdb.Txn) error {
		dbi, ok := db.GetDBis()[ConfigDBIName]
		if !ok {
			return fmt.Errorf("DBI %q not found", ConfigDBIName)
		}

		var cfg Configuration
		if err := TxnGetAndUnmarshal(txn, dbi, []byte(ConfigDataKey), &cfg); err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		if err := updateFunc(&cfg); err != nil {
			return fmt.Errorf("update function failed: %w", err)
		}

		if err := TxnMarshalAndPut(txn, dbi, []byte(ConfigDataKey), cfg); err != nil {
			return fmt.Errorf("failed to update config: %w", err)
		}

		return nil
	})
}
