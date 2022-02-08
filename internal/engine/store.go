package engine

import (
	"bytes"
	"context"
	"fmt"
	"log"
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

type Store struct {
	db     *badger.DB
	prefix string
}

func (store *Store) List(ctx context.Context, base interface{}) (results []interface{}, err error) {
	txn := store.db.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte(store.prefix)
	opts.PrefetchValues = true

	iter := txn.NewIterator(opts)
	defer iter.Close()

	iter.Seek(opts.Prefix)

	for ; iter.ValidForPrefix(opts.Prefix); iter.Next() {
		err = iter.Item().Value(func(val []byte) error {
			v := reflect.New(reflect.TypeOf(base)).Interface()
			log.Printf("%#v", v)

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

func (store *Store) Delete(ctx context.Context, kind, name string) (err error) {
	key := []byte(fmt.Sprintf("%s/%s/%s", store.prefix, kind, name))

	txn := extractTxn(ctx)
	if txn == nil {
		txn = &Txn{store.db.NewTransaction(true)}
		defer txn.CommitOrDiscard(&err)
	}

	return txn.txn.Delete(key)
}
