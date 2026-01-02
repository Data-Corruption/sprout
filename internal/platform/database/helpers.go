package database

import (
	"encoding/json"
	"fmt"
	"time"

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

// Generic Helpers ------------------------------------------------------------

// View retrieves a copy of a value from the database.
// lmdb.IsNotFound(err) will be true if the key was not found.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func View[T any](db *wrap.DB, dbiName string, key []byte) (*T, error) {
	data, err := db.Read(dbiName, key)
	if err != nil {
		return nil, err
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

// Upsert updates a value in the database using the provided update function,
// creating it with defaultFn if it does not exist.
// Returns true if the value was created.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func Upsert[T any](db *wrap.DB, dbiName string, key []byte, defaultFn func() T, updateFn func(*T) error) (bool, error) {
	created := false

	if err := db.Update(func(txn *lmdb.Txn) error {
		dbi, ok := db.GetDBis()[dbiName]
		if !ok {
			return fmt.Errorf("DBI %q not found", dbiName)
		}

		var value T
		err := TxnGetAndUnmarshal(txn, dbi, key, &value)
		if err != nil {
			if !lmdb.IsNotFound(err) {
				return fmt.Errorf("failed to get value: %w", err)
			}
			created = true
			value = defaultFn()
		}

		if err := updateFn(&value); err != nil {
			return fmt.Errorf("update function failed: %w", err)
		}

		if err := TxnMarshalAndPut(txn, dbi, key, value); err != nil {
			return fmt.Errorf("failed to update value: %w", err)
		}

		return nil
	}); err != nil {
		return false, err
	}

	return created, nil
}

// ForEachAction specifies what to do with an entry after the callback.
type ForEachAction int

const (
	Keep   ForEachAction = iota // no changes to entry
	Update                      // re-marshal and store entry
	Delete                      // remove entry
)

// ForEach iterates over all entries in a DBI, applying the callback to each.
// The callback receives the key and a pointer to the unmarshaled value.
// Return (Keep, nil) to leave unchanged, (Update, nil) to save changes, (Delete, nil) to remove.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func ForEach[T any](db *wrap.DB, dbiName string, callback func(key []byte, value *T) (ForEachAction, error)) error {
	return db.Update(func(txn *lmdb.Txn) error {
		dbi, ok := db.GetDBis()[dbiName]
		if !ok {
			return fmt.Errorf("DBI %q not found", dbiName)
		}

		cursor, err := txn.OpenCursor(dbi)
		if err != nil {
			return fmt.Errorf("failed to create cursor: %w", err)
		}
		defer cursor.Close()

		for {
			k, v, err := cursor.Get(nil, nil, lmdb.Next)
			if lmdb.IsNotFound(err) {
				break // no more entries
			}
			if err != nil {
				return fmt.Errorf("failed to get next entry: %w", err)
			}

			var value T
			if err := json.Unmarshal(v, &value); err != nil {
				return fmt.Errorf("failed to unmarshal entry: %w", err)
			}

			action, err := callback(k, &value)
			if err != nil {
				return fmt.Errorf("callback failed: %w", err)
			}

			switch action {
			case Update:
				if err := TxnMarshalAndPut(txn, dbi, k, value); err != nil {
					return fmt.Errorf("failed to update entry: %w", err)
				}
			case Delete:
				if err := cursor.Del(0); err != nil {
					return fmt.Errorf("failed to delete entry: %w", err)
				}
			}
		}
		return nil
	})
}

// Type-Specific Wrappers -----------------------------------------------------

// ViewConfig retrieves a copy of the current configuration from the database.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func ViewConfig(db *wrap.DB) (*Configuration, error) {
	return View[Configuration](db, ConfigDBIName, []byte(ConfigDataKey))
}

func defaultConfig() Configuration {
	return Configuration{
		LogLevel:            "WARN",
		Port:                DefaultPort,
		Host:                "localhost",
		UpdateNotifications: true,
		LastUpdateCheck:     time.Now(),
	}
}

// UpdateConfig updates the configuration in the database using the provided update function.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func UpdateConfig(db *wrap.DB, updateFunc func(cfg *Configuration) error) error {
	_, err := Upsert(db, ConfigDBIName, []byte(ConfigDataKey), defaultConfig, updateFunc)
	return err
}
