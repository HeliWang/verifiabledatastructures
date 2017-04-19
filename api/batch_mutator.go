/*

Copyright 2017 Continusec Pty Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package api

import (
	"bytes"
	"encoding/hex"
	"log"
	"time"

	"github.com/continusec/verifiabledatastructures/pb"
	"github.com/golang/protobuf/proto"
)

// BatchMutatorConfig has configuration data for a batch mutator service
type BatchMutatorConfig struct {
	// Writer is the underlying database to write to
	Writer StorageWriter

	// Timeout is the max time to hold buffered content before writing out
	Timeout time.Duration

	// BatchSize is the max number of items to buffer before writing out
	BatchSize int

	// BufferSize is how big a channel to hold mutations before blocking
	BufferSize int
}

// CreateBatchMutator creates a mutator for the given database that batches stuff up and periodically writes it
func CreateBatchMutator(conf *BatchMutatorConfig) MutatorService {
	rv := &batchMutatorImpl{
		Conf: conf,
		Ch:   make(chan *chObject, conf.BufferSize),
	}
	go rv.consume()
	return rv
}

type chObject struct {
	ns  []byte
	mut *pb.Mutation
}

type op struct {
	Key, Bucket []byte
	Value       proto.Message
}

type mapNoLockDB struct {
	M      map[string][]byte
	Parent KeyReader
	L      []*op
}

type batchMutatorImpl struct {
	Conf *BatchMutatorConfig
	Ch   chan *chObject
}

// It must return nil, ErrNoSuchKey if none found
func (m *mapNoLockDB) Get(bucket, key []byte, value proto.Message) error {
	k := hex.EncodeToString(bucket) + "|" + hex.EncodeToString(key)
	b, ok := m.M[k]
	if ok {
		return proto.Unmarshal(b, value)
	}
	return m.Parent.Get(bucket, key, value)
}

// Set sets the thing. Value of nil means delete.
// It must return nil, ErrNoSuchKey if none found
func (m *mapNoLockDB) Set(bucket, key []byte, value proto.Message) error {
	k := hex.EncodeToString(bucket) + "|" + hex.EncodeToString(key)
	b, err := proto.Marshal(value)
	if err != nil {
		return err
	}
	m.M[k] = b
	m.L = append(m.L, &op{
		Bucket: bucket,
		Key:    key,
		Value:  value,
	})
	return nil
}

// Keep reading and apply until timeout or any other reason
func (bm *batchMutatorImpl) handleBatch(kw KeyWriter, startSize int64, seed *chObject) (int64, *chObject, error) {
	curSize := startSize
	var err error
	obj := seed
	var ok bool
	cnt := bm.Conf.BatchSize
	for {
		if !bytes.Equal(seed.ns, obj.ns) {
			return curSize, seed, nil
		}
		curSize, err = ApplyMutation(kw, curSize, obj.mut)
		if err != nil {
			return 0, nil, err
		}
		cnt--
		if cnt == 0 {
			return curSize, nil, nil
		}
		select {
		case obj, ok = <-bm.Ch:
			if !ok {
				return curSize, nil, nil
			}
		case <-time.After(bm.Conf.Timeout):
			return curSize, nil, nil
		}
	}
}

func (bm *batchMutatorImpl) consume() {
	var err error
	var seed *chObject
	var ok bool
	for {
		if seed == nil {
			seed, ok = <-bm.Ch
			if !ok {
				return
			}
		}

		wrapper := &mapNoLockDB{
			M: make(map[string][]byte),
		}
		var startSize, nextSize int64

		ns := seed.ns
		err := bm.Conf.Writer.ExecuteReadOnly(ns, func(kr KeyReader) error {
			wrapper.Parent = kr

			startSize, err = readObjectSize(kr)
			if err != nil {
				return err
			}

			nextSize, seed, err = bm.handleBatch(wrapper, startSize, seed)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			log.Fatal(err)
		}

		if nextSize > startSize { // save it out
			err = bm.Conf.Writer.ExecuteUpdate(ns, func(kw KeyWriter) error {
				for _, o := range wrapper.L {
					err := kw.Set(o.Bucket, o.Key, o.Value)
					if err != nil {
						return err
					}
				}
				return writeObjectSize(kw, nextSize)
			})
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

// QueueMutation applies the mutation, normally asynchronously, but synchronously for the InstantMutator
func (bm *batchMutatorImpl) QueueMutation(ns []byte, mut *pb.Mutation) error {
	bm.Ch <- &chObject{ns: ns, mut: mut}
	return nil
}
