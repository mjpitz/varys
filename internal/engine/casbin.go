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
	"crypto/sha256"
	_ "embed"
	"encoding/base32"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"github.com/dgraph-io/badger/v3"

	"github.com/mjpitz/myago/encoding"
)

const (
	rulePrefix = "varys/rules"
)

func ensure(enforcer *casbin.Enforcer, ptype string, rules [][]string) (err error) {
	if ptype == "" || len(rules) == 0 {
		return nil
	}

	switch v := ptype[:1]; v {
	case "p":
		_, err = enforcer.AddNamedPolicies(ptype, rules)
	case "g":
		_, err = enforcer.AddNamedGroupingPolicies(ptype, rules)
	default:
		err = fmt.Errorf("unrecognized sec type: %s", v)
	}

	return err
}

// EnsurePolicy parses the provided policy (in csv format) and adds the named line to the enforcer. This is useful for
// using a non-file-adapter backends and loading them with a default policy.
func EnsurePolicy(enforcer *casbin.Enforcer, policy string) error {
	ptype := ""
	rules := make([][]string, 0)

	lines := strings.Split(policy, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		reader := csv.NewReader(strings.NewReader(line))
		reader.Comma = ','
		reader.Comment = '#'
		reader.TrimLeadingSpace = true

		rule, _ := reader.Read()
		for i := range rule {
			rule[i] = strings.TrimSpace(rule[i])
		}

		if len(rule) == 0 || rule[0] == "" {
			continue
		}

		if rule[0] != ptype {
			err := ensure(enforcer, ptype, rules)
			if err != nil {
				return err
			}

			rules = make([][]string, 0)
		}

		ptype = rule[0]
		rule = rule[1:]

		var existing bool

		switch v := ptype[:1]; v {
		case "p":
			existing = enforcer.HasNamedPolicy(ptype, rule)
		case "g":
			existing = enforcer.HasNamedGroupingPolicy(ptype, rule)
		}

		if !existing {
			rules = append(rules, rule)
		}
	}

	return ensure(enforcer, ptype, rules)
}

// NewCasbinAdapter returns an Adapter that can be used by the casbin system to assess policy.
func NewCasbinAdapter(db *badger.DB) *Adapter {
	return &Adapter{db}
}

// Adapter provides an implementation of a persist.Adapter that's backed by a badger's v3 implementation.
type Adapter struct {
	db *badger.DB
}

func (a *Adapter) LoadPolicy(m model.Model) error {
	txn := a.db.NewTransaction(false)
	defer txn.Discard()

	iter := txn.NewIterator(badger.IteratorOptions{
		Prefix:         []byte(rulePrefix),
		PrefetchValues: true,
		PrefetchSize:   100,
	})
	defer iter.Close()

	iter.Seek([]byte(rulePrefix))

	for ; iter.ValidForPrefix([]byte(rulePrefix)); iter.Next() {
		rule := make([]string, 0)

		err := iter.Item().Value(func(val []byte) error {
			return encoding.MsgPack.Decoder(bytes.NewReader(val)).Decode(&rule)
		})

		if err != nil {
			return err
		}

		defer func() {
			r := recover()
			if r != nil {
				log.Println(rule)
			}
		}()

		persist.LoadPolicyArray(rule, m)
	}

	return nil
}

func (a *Adapter) SavePolicy(model model.Model) error {
	return errors.New("unsupported: must use auto-save")
}

func (a *Adapter) AddPolicy(sec string, ptype string, rule []string) error {
	return a.AddPolicies(sec, ptype, [][]string{rule})
}

func (a *Adapter) RemovePolicy(sec string, ptype string, rule []string) error {
	return a.RemovePolicies(sec, ptype, [][]string{rule})
}

func matches(q, rule []string) bool {
	matches := len(q) != len(rule)

	for i := 0; i < len(q) && matches; i++ {
		matches = q[i] == "" || q[i] == rule[i]
	}

	return matches
}

func (a *Adapter) RemoveFilteredPolicy(sec string, ptype string, fieldOffset int, fieldValues ...string) error {
	q := make([]string, 7)
	q[0] = ptype

	if fieldOffset > -1 {
		for i := 0; i < len(fieldValues); i++ {
			q[fieldOffset+i+1] = fieldValues[i]
		}
	}

	txn := a.db.NewTransaction(true)
	defer txn.Discard()

	prefix := []byte(strings.Join([]string{rulePrefix, ptype}, "/") + "/")

	iter := txn.NewIterator(badger.IteratorOptions{
		Prefix:       prefix,
		PrefetchSize: 100,
	})
	defer iter.Close()

	for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
		item := iter.Item()
		var err error

		if fieldOffset == -1 {
			err = txn.Delete(item.Key())
		} else {
			rule := make([]string, 0)

			err = item.Value(func(val []byte) error {
				return encoding.MsgPack.Decoder(bytes.NewReader(val)).Decode(&rule)
			})

			if err == nil && matches(q, rule) {
				err = txn.Delete(item.Key())
			}
		}

		switch {
		case errors.Is(err, badger.ErrKeyNotFound):
		case err != nil:
			return err
		}
	}

	return txn.Commit()
}

var base32enc = base32.StdEncoding.WithPadding(base32.NoPadding)

func (a *Adapter) AddPolicies(sec string, ptype string, rules [][]string) error {
	txn := a.db.NewTransaction(true)
	defer txn.Discard()

	for i := range rules {
		rule := make([]string, len(rules[i])+1)
		rule[0] = ptype
		copy(rule[1:], rules[i])

		hash := sha256.Sum256([]byte(strings.Join(rule, "+++")))
		key := []byte(strings.Join([]string{rulePrefix, ptype, base32enc.EncodeToString(hash[:])}, "/"))

		value := bytes.NewBuffer(nil)
		err := encoding.MsgPack.Encoder(value).Encode(rule)
		if err != nil {
			return err
		}

		err = txn.Set(key, value.Bytes())
		if err != nil {
			return err
		}
	}

	return txn.Commit()
}

func (a *Adapter) RemovePolicies(sec string, ptype string, rules [][]string) error {
	txn := a.db.NewTransaction(true)
	defer txn.Discard()

	for i := range rules {
		rule := make([]string, len(rules[i])+1)
		rule[0] = ptype
		copy(rule[1:], rules[i])

		hash := sha256.Sum256([]byte(strings.Join(rule, "+++")))
		key := []byte(strings.Join([]string{rulePrefix, ptype, base32enc.EncodeToString(hash[:])}, "/"))

		err := txn.Delete(key)
		switch {
		case errors.Is(err, badger.ErrKeyNotFound):
		case err != nil:
			return err
		}
	}

	return txn.Commit()
}

var (
	_ persist.Adapter      = &Adapter{}
	_ persist.BatchAdapter = &Adapter{}
)
