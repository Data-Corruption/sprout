package helpers

import (
	"context"
	"encoding/json"
	"errors"
	"sprout/go/database"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/lmdb-go/wrap"
)

// basic txn ops

func MarshalAndPut(txn *lmdb.Txn, dbi lmdb.DBI, key []byte, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := txn.Put(dbi, key, data, 0); err != nil {
		return err
	}
	return nil
}

// lmdb.IsNotFound(err) will be true if the key was not found in the database.
func GetAndUnmarshal(txn *lmdb.Txn, dbi lmdb.DBI, key []byte, value any) error {
	buf, err := txn.Get(dbi, key)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(buf, value); err != nil {
		return err
	}
	return nil
}

// getting db stuff

func GetDbAndDBI(ctx context.Context, dbiName string) (*wrap.DB, lmdb.DBI, error) {
	db := database.FromContext(ctx)
	if db == nil {
		return nil, 0, errors.New("database not found in context")
	}
	dbi, ok := db.GetDBis()[dbiName]
	if !ok {
		return nil, 0, errors.New("DBI not found in database: " + dbiName)
	}
	return db, dbi, nil
}
