package database

import (
	"encoding/json"
	"fmt"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/lmdb-go/wrap"
)

// ForEachAction specifies what to do with an entry after the callback.
type ForEachAction int

const (
	ActionKeep   ForEachAction = iota // no changes to entry
	ActionUpdate                      // re-marshal and store entry
	ActionDelete                      // remove entry
)

// =============================================================================
// Transaction-based helpers (use these when composing multiple operations)
// =============================================================================

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

// TxnView retrieves a copy of a value from the database within an existing transaction.
// lmdb.IsNotFound(err) will be true if the key was not found.
func TxnView[T any](txn *lmdb.Txn, dbi lmdb.DBI, key []byte) (*T, error) {
	var value T
	if err := TxnGetAndUnmarshal(txn, dbi, key, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

// TxnPut marshals and stores a value in the database within an existing transaction.
func TxnPut[T any](txn *lmdb.Txn, dbi lmdb.DBI, key []byte, value T) error {
	return TxnMarshalAndPut(txn, dbi, key, value)
}

// TxnDeleteKey removes a key from the database within an existing transaction.
// Returns nil if key doesn't exist (idempotent).
func TxnDeleteKey(txn *lmdb.Txn, dbi lmdb.DBI, key []byte) error {
	err := txn.Del(dbi, key, nil)
	if lmdb.IsNotFound(err) {
		return nil // Idempotent
	}
	return err
}

// TxnViewAll retrieves all entries from a DBI as a slice within an existing transaction.
// Useful for small DBIs like roles where you need all entries.
//
// The optional filter function receives raw key/value bytes BEFORE unmarshalling.
// Return true to include the entry, false to skip it. Pass nil to include all entries.
// This is useful for key prefix filtering without the cost of unmarshalling skipped entries.
func TxnViewAll[T any](txn *lmdb.Txn, dbi lmdb.DBI, filter func(key, value []byte) bool) ([]T, error) {
	var result []T

	cursor, err := txn.OpenCursor(dbi)
	if err != nil {
		return nil, fmt.Errorf("failed to create cursor: %w", err)
	}
	defer cursor.Close()

	// Start at first entry
	k, v, err := cursor.Get(nil, nil, lmdb.First)
	for ; !lmdb.IsNotFound(err); k, v, err = cursor.Get(nil, nil, lmdb.Next) {
		if err != nil {
			return nil, fmt.Errorf("failed to get entry: %w", err)
		}

		// Apply filter if provided
		if filter != nil && !filter(k, v) {
			continue
		}

		var value T
		if err := json.Unmarshal(v, &value); err != nil {
			return nil, fmt.Errorf("failed to unmarshal entry: %w", err)
		}
		result = append(result, value)
	}
	return result, nil
}

// TxnUpsert updates a value in the database using the provided update function,
// creating it with defaultFn if it does not exist.
// Returns true if the value was created.
func TxnUpsert[T any](txn *lmdb.Txn, dbi lmdb.DBI, key []byte, defaultFn func() T, updateFn func(*T) error) (bool, error) {
	created := false

	var value T
	err := TxnGetAndUnmarshal(txn, dbi, key, &value)
	if err != nil {
		if !lmdb.IsNotFound(err) {
			return false, fmt.Errorf("failed to get value: %w", err)
		}
		created = true
		value = defaultFn()
	}

	if err := updateFn(&value); err != nil {
		return false, fmt.Errorf("update function failed: %w", err)
	}

	if err := TxnMarshalAndPut(txn, dbi, key, value); err != nil {
		return false, fmt.Errorf("failed to update value: %w", err)
	}

	return created, nil
}

// TxnUpdate updates a value in the database using the provided update function.
func TxnUpdate[T any](txn *lmdb.Txn, dbi lmdb.DBI, key []byte, updateFn func(*T) error) error {
	var value T
	err := TxnGetAndUnmarshal(txn, dbi, key, &value)
	if err != nil {
		return fmt.Errorf("failed to get value: %w", err)
	}

	if err := updateFn(&value); err != nil {
		return fmt.Errorf("update function failed: %w", err)
	}

	if err := TxnMarshalAndPut(txn, dbi, key, value); err != nil {
		return fmt.Errorf("failed to update value: %w", err)
	}

	return nil
}

// TxnForEach iterates over all entries in a DBI within an existing transaction, applying the callback to each.
// The callback receives the key and a pointer to the unmarshaled value.
// Return (Keep, nil) to leave unchanged, (Update, nil) to save changes, (Delete, nil) to remove.
//
// The optional filter function receives raw key/value bytes BEFORE unmarshalling.
// Return true to process the entry, false to skip it. Pass nil to process all entries.
// This is useful for key prefix filtering without the cost of unmarshalling skipped entries.
func TxnForEach[T any](txn *lmdb.Txn, dbi lmdb.DBI, filter func(key, value []byte) bool, callback func(key []byte, value *T) (ForEachAction, error)) error {
	cursor, err := txn.OpenCursor(dbi)
	if err != nil {
		return fmt.Errorf("failed to create cursor: %w", err)
	}
	defer cursor.Close()

	// Start at first entry
	k, v, err := cursor.Get(nil, nil, lmdb.First)
	for ; !lmdb.IsNotFound(err); k, v, err = cursor.Get(nil, nil, lmdb.Next) {
		if err != nil {
			return fmt.Errorf("failed to get entry: %w", err)
		}

		// Apply filter if provided
		if filter != nil && !filter(k, v) {
			continue
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
		case ActionUpdate:
			data, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal entry: %w", err)
			}
			if err := cursor.Put(k, data, lmdb.Current); err != nil {
				return fmt.Errorf("failed to update entry: %w", err)
			}
		case ActionDelete:
			if err := cursor.Del(0); err != nil {
				return fmt.Errorf("failed to delete entry: %w", err)
			}
		}
	}
	return nil
}

// =============================================================================
// Convenience wrappers (start their own transaction - don't nest these)
// =============================================================================

// View retrieves a copy of a value from the database.
// lmdb.IsNotFound(err) will be true if the key was not found.
//
// WARNING: Starts a transaction. Use TxnView if you need to compose multiple operations.
func View[T any](db *wrap.DB, dbi lmdb.DBI, key []byte) (*T, error) {
	var value T
	err := db.View(func(txn *lmdb.Txn) error {
		return TxnGetAndUnmarshal(txn, dbi, key, &value)
	})
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// Put marshals and stores a value in the database.
//
// WARNING: Starts a transaction. Use TxnPut if you need to compose multiple operations.
// If an error is returned, the transaction is rolled back and nothing is persisted.
func Put[T any](db *wrap.DB, dbi lmdb.DBI, key []byte, value T) error {
	return db.Update(func(txn *lmdb.Txn) error {
		return TxnMarshalAndPut(txn, dbi, key, value)
	})
}

// DeleteKey removes a key from the database.
// Returns nil if key doesn't exist (idempotent).
//
// WARNING: Starts a transaction. Use TxnDeleteKey if you need to compose multiple operations.
// If an error is returned, the transaction is rolled back and nothing is persisted.
func DeleteKey(db *wrap.DB, dbi lmdb.DBI, key []byte) error {
	return db.Update(func(txn *lmdb.Txn) error {
		return TxnDeleteKey(txn, dbi, key)
	})
}

// ViewAll retrieves all entries from a DBI as a slice.
// Useful for small DBIs like roles where you need all entries.
//
// The optional filter function receives raw key/value bytes BEFORE unmarshalling.
// Return true to include the entry, false to skip it. Pass nil to include all entries.
// This is useful for key prefix filtering without the cost of unmarshalling skipped entries.
//
// WARNING: Starts a transaction. Use TxnViewAll if you need to compose multiple operations.
func ViewAll[T any](db *wrap.DB, dbi lmdb.DBI, filter func(key, value []byte) bool) ([]T, error) {
	var result []T
	err := db.View(func(txn *lmdb.Txn) error {
		var err error
		result, err = TxnViewAll[T](txn, dbi, filter)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Upsert updates a value in the database using the provided update function,
// creating it with defaultFn if it does not exist.
// Returns true if the value was created.
//
// WARNING: Starts a transaction. Use TxnUpsert if you need to compose multiple operations.
// If updateFn returns an error, the transaction is rolled back and nothing is persisted.
func Upsert[T any](db *wrap.DB, dbi lmdb.DBI, key []byte, defaultFn func() T, updateFn func(*T) error) (bool, error) {
	var created bool
	err := db.Update(func(txn *lmdb.Txn) error {
		var err error
		created, err = TxnUpsert(txn, dbi, key, defaultFn, updateFn)
		return err
	})
	return created, err
}

// Update updates a value in the database using the provided update function.
//
// WARNING: Starts a transaction. Use TxnUpdate if you need to compose multiple operations.
// If updateFn returns an error, the transaction is rolled back and nothing is persisted.
func Update[T any](db *wrap.DB, dbi lmdb.DBI, key []byte, updateFn func(*T) error) error {
	return db.Update(func(txn *lmdb.Txn) error {
		return TxnUpdate(txn, dbi, key, updateFn)
	})
}

// ForEach iterates over all entries in a DBI, applying the callback to each.
// The callback receives the key and a pointer to the unmarshaled value.
// Return (Keep, nil) to leave unchanged, (Update, nil) to save changes, (Delete, nil) to remove.
//
// The optional filter function receives raw key/value bytes BEFORE unmarshalling.
// Return true to process the entry, false to skip it. Pass nil to process all entries.
// This is useful for key prefix filtering without the cost of unmarshalling skipped entries.
//
// WARNING: Starts a transaction. Use TxnForEach if you need to compose multiple operations.
// If the callback returns a non-nil error, the transaction is rolled back and nothing is persisted.
func ForEach[T any](db *wrap.DB, dbi lmdb.DBI, filter func(key, value []byte) bool, callback func(key []byte, value *T) (ForEachAction, error)) error {
	return db.Update(func(txn *lmdb.Txn) error {
		return TxnForEach(txn, dbi, filter, callback)
	})
}
