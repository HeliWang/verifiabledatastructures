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

package kvstore

import (
	"time"

	"github.com/boltdb/bolt"
	"github.com/continusec/vds-server/api"
	"github.com/continusec/vds-server/pb"
)

// BoltBackedService gives a service that persists to a BoltDB file.
type BoltBackedService struct {
	Path     string
	Accounts []*pb.Account

	db *bolt.DB
}

// Init must be called before use
func (bbs *BoltBackedService) Init() error {
	var err error
	bbs.db, err = bolt.Open(bbs.Path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	return err
}

// CreateClient returns a client to the BoltBackedService.
func (bbs *BoltBackedService) CreateClient(account int64, auth *api.AuthorizationContext) (api.VerifiableDataStructuresService, error) {
	return &bbClient{
		service: bbs,
		auth:    auth,
		account: account,
	}, nil
}

type bbClient struct {
	service *BoltBackedService
	auth    *api.AuthorizationContext
	account int64
}
