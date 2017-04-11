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

import "github.com/continusec/verifiabledatastructures/client"

type LocalService struct {
	Mutator      MutatorService
	AccessPolicy AuthorizationOracle
	Reader       StorageReader
}

func (ls *LocalService) Account(account string, apiKey string) client.Account {
	return &serverAccount{
		Service: ls,
		Account: account,
		APIKey:  apiKey,
	}
}