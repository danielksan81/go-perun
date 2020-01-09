// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package db

import (
	"github.com/pkg/errors"

	"perun.network/go-perun/db/key"
	"perun.network/go-perun/log"
)

// Table is a wrapper around a database with a key prefix. All key access is
// automatically prefixed. Close() is a noop and properties are forwarded
// from the database.
type table struct {
	Database
	prefix string
}

// NewTable creates a new table.
func NewTable(db Database, prefix string) Database {
	if db == nil {
		log.Panic("database must not be nil")
	}

	return &table{
		Database: db,
		prefix:   prefix,
	}
}

func (t *table) pkey(key string) string {
	return t.prefix + key
}

// Has calls db.Has with the prefixed key.
func (t *table) Has(key string) (bool, error) {
	return t.Database.Has(t.pkey(key))
}

// Get calls db.Get with the prefixed key.
func (t *table) Get(key string) (string, error) {
	return t.Database.Get(t.pkey(key))
}

// GetBytes calls db.GetBytes with the prefixed key.
func (t *table) GetBytes(key string) ([]byte, error) {
	return t.Database.GetBytes(t.pkey(key))
}

// Put calls db.Put with the prefixed key.
func (t *table) Put(key, value string) error {
	return t.Database.Put(t.pkey(key), value)
}

// PutBytes calls db.PutBytes with the prefixed key.
func (t *table) PutBytes(key string, value []byte) error {
	if value == nil {
		return errors.New("value must not be nil")
	}
	return t.Database.PutBytes(t.pkey(key), value)
}

// Delete calls db.Delete with the prefixed key.
func (t *table) Delete(key string) error {
	return t.Database.Delete(t.pkey(key))
}

// NewBatch creates a new batch.
func (t *table) NewBatch() Batch {
	return &tableBatch{t.Database.NewBatch(), t.prefix}
}

// NewIterator creates a new table iterator.
func (t *table) NewIterator() Iterator {
	return newTableIterator(t.Database.NewIteratorWithPrefix(t.prefix), t)
}

// NewIteratorWithRange creates a new ranged iterator.
func (t *table) NewIteratorWithRange(start string, end string) Iterator {
	start = t.pkey(start)
	if end == "" {
		end = key.IncPrefix(t.prefix)
	} else {
		end = t.pkey(end)
	}

	return newTableIterator(t.Database.NewIteratorWithRange(start, end), t)
}

// NewIteratorWithPrefix creates a new iterator for a prefix.
func (t *table) NewIteratorWithPrefix(prefix string) Iterator {
	return newTableIterator(t.Database.NewIteratorWithPrefix(t.pkey(prefix)), t)
}
