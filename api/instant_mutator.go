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
	"log"

	"github.com/continusec/verifiabledatastructures/pb"
	"github.com/golang/protobuf/proto"
)

type InstantMutator struct {
	Writer StorageWriter
}

func (m *InstantMutator) QueueMutation(ns []byte, mut *pb.Mutation) (MutatorPromise, error) {
	return &instancePromise{Err: m.Writer.ExecuteUpdate(ns, func(kw KeyWriter) error {
		log.Printf("Instant mutation start: %s\n", proto.CompactTextString(mut))
		rv := ApplyMutation(kw, mut)
		log.Printf("Instant mutation end: %s\n", rv)
		return rv
	})}, nil
}

type instancePromise struct {
	Err error
}

func (i *instancePromise) WaitUntilDone() error {
	return i.Err
}