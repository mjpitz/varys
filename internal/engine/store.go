// Copyright (C) 2022  Mya Pitzeruse
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//

package engine

import (
	"bytes"
	"context"
	"fmt"
	"reflect"

	"github.com/dgraph-io/badger/v3"

	"github.com/mjpitz/myago"
	"github.com/mjpitz/myago/encoding"
)

const txnContextKey = myago.ContextKey("badger.txn")

func withTxn(ctx context.Context, txn *Txn) context.Context {
	return context.WithValue(ctx, txnContextKey, txn)
}

func extractTxn(ctx context.Context) *Txn {
	val := ctx.Value(txnContextKey)
	if val == nil {
		return nil
	}

	return val.(*Txn)
}

type Txn struct {
	txn *badger.Txn
}

func (txn *Txn) CommitOrDiscard(err *error) {
	defer txn.txn.Discard()

	if *err != nil {
		return
	}

	*err = txn.txn.Commit()
}

// Store provides common CRUD operations on top of badgerdb. Operations are scoped to a prefix, allowing multiple
// resources to be managed by the same database.
type Store struct {
	db     *badger.DB
	prefix string
}

// List objects within the store.
func (store *Store) List(ctx context.Context, base interface{}) (results []interface{}, err error) {
	txn := store.db.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte(store.prefix)
	opts.PrefetchValues = true

	iter := txn.NewIterator(opts)
	defer iter.Close()

	for iter.Seek(opts.Prefix); iter.ValidForPrefix(opts.Prefix); iter.Next() {
		err = iter.Item().Value(func(val []byte) error {
			v := reflect.New(reflect.TypeOf(base)).Interface()

			err = encoding.MsgPack.Decoder(bytes.NewReader(val)).Decode(&v)
			if err != nil {
				return err
			}

			results = append(results, v)
			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

// Put an object in the store.
func (store *Store) Put(ctx context.Context, kind, name string, v interface{}) (err error) {
	key := []byte(fmt.Sprintf("%s/%s/%s", store.prefix, kind, name))
	value := bytes.NewBuffer(nil)

	err = encoding.MsgPack.Encoder(value).Encode(v)
	if err != nil {
		return
	}

	txn := extractTxn(ctx)
	if txn == nil {
		txn = &Txn{store.db.NewTransaction(true)}
		defer txn.CommitOrDiscard(&err)
	}

	return txn.txn.Set(key, value.Bytes())
}

// Get an object from the store.
func (store *Store) Get(ctx context.Context, kind, name string, v interface{}) (err error) {
	key := []byte(fmt.Sprintf("%s/%s/%s", store.prefix, kind, name))

	txn := extractTxn(ctx)
	if txn == nil {
		txn = &Txn{store.db.NewTransaction(false)}
		defer txn.CommitOrDiscard(&err)
	}

	item, err := txn.txn.Get(key)
	if err != nil {
		return
	}

	return item.Value(func(val []byte) error {
		return encoding.MsgPack.Decoder(bytes.NewReader(val)).Decode(v)
	})
}

// Delete an object from the store.
func (store *Store) Delete(ctx context.Context, kind, name string) (err error) {
	key := []byte(fmt.Sprintf("%s/%s/%s", store.prefix, kind, name))

	txn := extractTxn(ctx)
	if txn == nil {
		txn = &Txn{store.db.NewTransaction(true)}
		defer txn.CommitOrDiscard(&err)
	}

	return txn.txn.Delete(key)
}
