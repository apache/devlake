/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"

	"github.com/apache/devlake/core/errors"
)

// The server holds an exclusive DB lock across its lifetime. This additional
// process-local lock rejects overlapping collectors using the same raw scope.
var graphqlScopes = struct {
	sync.Mutex
	active map[string]bool
}{active: map[string]bool{}}

func acquireGraphqlScope(table, params string) (func(), errors.Error) {
	key := table + "\x00" + params
	graphqlScopes.Lock()
	defer graphqlScopes.Unlock()
	if graphqlScopes.active[key] {
		return nil, errors.Default.New("GraphQL raw scope is already being collected")
	}
	graphqlScopes.active[key] = true
	return func() { graphqlScopes.Lock(); delete(graphqlScopes.active, key); graphqlScopes.Unlock() }, nil
}

var graphqlBinaryIdentity = sync.OnceValues(func() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
})

// Stage inputs to disk before fetching. The digest makes changed ordering,
// batching or input content an explicit error instead of duplicating old data.
// SQL cursors are closed before collection. Payloads stay on disk; only types
// and fixed-size hashes for duplicate detection are retained in memory.
type graphqlStagedInput struct {
	file    *os.File
	decoder *json.Decoder
	types   []reflect.Type
	pending *graphqlInputRecord
	err     error
}
type graphqlInputRecord struct {
	Type int
	Data json.RawMessage
}

func stageGraphqlInput(ctx context.Context, source Iterator) (staged *graphqlStagedInput, digest string, resultErr errors.Error) {
	defer func() {
		if closeErr := source.Close(); resultErr == nil && closeErr != nil {
			resultErr = closeErr
		}
		if resultErr != nil && staged != nil {
			_ = staged.Close()
			staged = nil
		}
	}()
	file, err := os.CreateTemp("", "devlake-graphql-input-*")
	if err != nil {
		return nil, "", errors.Convert(err)
	}
	staged = &graphqlStagedInput{file: file}
	hash := sha256.New()
	encoder := json.NewEncoder(io.MultiWriter(file, hash))
	typeIDs := map[reflect.Type]int{}
	seen := map[[32]byte]bool{}
	for source.HasNext() {
		if err := ctx.Err(); err != nil {
			return staged, "", errors.Convert(err)
		}
		value, err := source.Fetch()
		if err != nil {
			return staged, "", err
		}
		typ := reflect.TypeOf(value)
		id, found := typeIDs[typ]
		if !found {
			id = len(staged.types)
			typeIDs[typ] = id
			staged.types = append(staged.types, typ)
			// A type/schema change must not silently reuse an old manifest.
			_, _ = fmt.Fprintf(hash, "%v\n", typ)
		}
		data, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			return staged, "", errors.Convert(marshalErr)
		}
		key := sha256.Sum256(data)
		if seen[key] {
			return staged, "", errors.BadInput.New("duplicate GraphQL input; provide unique inputs")
		}
		seen[key] = true
		if err := encoder.Encode(graphqlInputRecord{Type: id, Data: data}); err != nil {
			return staged, "", errors.Convert(err)
		}
	}
	if sourceErr, ok := source.(interface{ Err() error }); ok {
		if err := sourceErr.Err(); err != nil {
			return staged, "", errors.Convert(err)
		}
	}
	if err := ctx.Err(); err != nil {
		return staged, "", errors.Convert(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return staged, "", errors.Convert(err)
	}
	staged.decoder = json.NewDecoder(file)
	return staged, hex.EncodeToString(hash.Sum(nil)), nil
}
func (s *graphqlStagedInput) HasNext() bool {
	if s.err != nil {
		return false
	}
	var record graphqlInputRecord
	if err := s.decoder.Decode(&record); err != nil {
		if err != io.EOF {
			s.err = err
		}
		return false
	}
	s.pending = &record
	return true
}
func (s *graphqlStagedInput) Fetch() (interface{}, errors.Error) {
	if s.pending == nil {
		return nil, errors.Default.New("input Fetch without HasNext")
	}
	record := s.pending
	s.pending = nil
	typ := s.types[record.Type]
	if typ == nil {
		return nil, nil
	}
	value := reflect.New(typ)
	if err := json.Unmarshal(record.Data, value.Interface()); err != nil {
		return nil, errors.Convert(err)
	}
	return value.Elem().Interface(), nil
}
func (s *graphqlStagedInput) Err() error { return s.err }
func (s *graphqlStagedInput) Close() errors.Error {
	if s.file == nil {
		return nil
	}
	file := s.file
	s.file = nil
	closeErr := file.Close()
	removeErr := os.Remove(file.Name())
	if closeErr != nil {
		return errors.Convert(closeErr)
	}
	return errors.Convert(removeErr)
}
