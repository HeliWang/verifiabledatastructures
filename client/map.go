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

package client

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// VerifiableMap is an object used to interact with Verifiable Maps. To construct this
// object, call NewClient(...).VerifiableMap("mapname")
type verifiableMapImpl struct {
	Client  *Client
	MapName string
}

func (self *verifiableMapImpl) Name() string {
	return self.MapName
}

// MutationLog returns a pointer to the underlying Verifiable Log that represents
// a log of mutations to this map. Since this Verifiable Log is managed by this map,
// the log returned cannot be directly added to (to mutate, call Set and Delete methods
// on the map), however all read-only functions are present.
func (self *verifiableMapImpl) MutationLog() VerifiableLog {
	return &verifiableLogImpl{
		Client: self.Client.WithChildPath("/log/mutation"),
	}
}

// TreeHeadLog returns a pointer to the underlying Verifiable Log that represents
// a log of tree heads generated by this map. Since this Verifiable Map is managed by this map,
// the log returned cannot be directly added to however all read-only functions are present.
func (self *verifiableMapImpl) TreeHeadLog() VerifiableLog {
	return &verifiableLogImpl{
		Client: self.Client.WithChildPath("/log/treehead"),
	}
}

// Create will send an API call to create a new map with the name specified when the
// VerifiableMap object was instantiated.
func (self *verifiableMapImpl) Create() error {
	_, _, err := self.Client.MakeRequest("PUT", "", nil, nil)
	if err != nil {
		return err
	}
	return nil
}

// Destroy will send an API call to delete this map - this operation removes it permanently,
// and renders the name unusable again within the same account, so please use with caution.
func (self *verifiableMapImpl) Destroy() error {
	_, _, err := self.Client.MakeRequest("DELETE", "", nil, nil)
	if err != nil {
		return err
	}
	return nil
}

func parseHeadersForProof(headers http.Header) ([][]byte, error) {
	prv := make([][]byte, 256)
	actualHeaders, ok := headers[http.CanonicalHeaderKey("X-Verified-Proof")]
	if ok {
		for _, h := range actualHeaders {
			for _, commad := range strings.Split(h, ",") {
				bits := strings.SplitN(commad, "/", 2)
				if len(bits) == 2 {
					idx, err := strconv.Atoi(strings.TrimSpace(bits[0]))
					if err != nil {
						return nil, err
					}
					bs, err := hex.DecodeString(strings.TrimSpace(bits[1]))
					if err != nil {
						return nil, err
					}
					if idx < 256 {
						prv[idx] = bs
					}
				}
			}
		}
	}
	return prv, nil
}

// Get will return the value for the given key at the given treeSize. Pass continusec.Head
// to always get the latest value. factory is normally one of RawDataEntryFactory, JsonEntryFactory or RedactedJsonEntryFactory.
//
// Clients normally instead call VerifiedGet() with a MapTreeHead returned by VerifiedLatestMapState as this will also perform verification of inclusion.
func (self *verifiableMapImpl) Get(key []byte, treeSize int64, factory VerifiableEntryFactory) (*MapInclusionProof, error) {
	value, headers, err := self.Client.MakeRequest("GET", fmt.Sprintf("/tree/%d/key/h/%s%s", treeSize, hex.EncodeToString(key), factory.Format()), nil, nil)
	if err != nil {
		return nil, err
	}

	prv, err := parseHeadersForProof(headers)
	if err != nil {
		return nil, err
	}

	rv, err := factory.CreateFromBytes(value)
	if err != nil {
		return nil, err
	}

	vts, err := strconv.Atoi(headers.Get("X-Verified-TreeSize"))
	if err != nil {
		return nil, err
	}

	return &MapInclusionProof{
		Value:     rv,
		TreeSize:  int64(vts),
		AuditPath: prv,
		Key:       key,
	}, nil
}

// Set will generate a map mutation to set the given value for the given key.
// While this will return quickly, the change will be reflected asynchronously in the map.
// Returns an AddEntryResponse which contains the leaf hash for the mutation log entry.
func (self *verifiableMapImpl) Set(key []byte, value UploadableEntry) (*AddEntryResponse, error) {
	data, err := value.DataForUpload()
	if err != nil {
		return nil, err
	}
	contents, _, err := self.Client.MakeRequest("PUT", "/key/h/"+hex.EncodeToString(key)+value.Format(), data, nil)
	if err != nil {
		return nil, err
	}
	var aer JSONAddEntryResponse
	err = json.Unmarshal(contents, &aer)
	if err != nil {
		return nil, err
	}
	return &AddEntryResponse{EntryLeafHash: aer.Hash}, nil
}

// Update will generate a map mutation to set the given value for the given key, conditional on the
// previous leaf hash being that specified by previousLeaf.
// While this will return quickly, the change will be reflected asynchronously in the map.
// Returns an AddEntryResponse which contains the leaf hash for the mutation log entry.
func (self *verifiableMapImpl) Update(key []byte, value UploadableEntry, previousLeaf MerkleTreeLeaf) (*AddEntryResponse, error) {
	data, err := value.DataForUpload()
	if err != nil {
		return nil, err
	}
	prevLF, err := previousLeaf.LeafHash()
	if err != nil {
		return nil, err
	}

	contents, _, err := self.Client.MakeRequest("PUT", "/key/h/"+hex.EncodeToString(key)+value.Format(), data, [][2]string{
		[2]string{"X-Previous-LeafHash", hex.EncodeToString(prevLF)},
	})
	if err != nil {
		return nil, err
	}
	var aer JSONAddEntryResponse
	err = json.Unmarshal(contents, &aer)
	if err != nil {
		return nil, err
	}
	return &AddEntryResponse{EntryLeafHash: aer.Hash}, nil
}

// Delete will set generate a map mutation to delete the value for the given key. Calling Delete
// is equivalent to calling Set with an empty value.
// While this will return quickly, the change will be reflected asynchronously in the map.
// Returns an AddEntryResponse which contains the leaf hash for the mutation log entry.
func (self *verifiableMapImpl) Delete(key []byte) (*AddEntryResponse, error) {
	contents, _, err := self.Client.MakeRequest("DELETE", "/key/h/"+hex.EncodeToString(key), nil, nil)
	if err != nil {
		return nil, err
	}
	var aer JSONAddEntryResponse
	err = json.Unmarshal(contents, &aer)
	if err != nil {
		return nil, err
	}
	return &AddEntryResponse{EntryLeafHash: aer.Hash}, nil
}

// TreeHead returns map root hash for the map at the given tree size. Specify continusec.Head
// to receive a root hash for the latest tree size.
func (self *verifiableMapImpl) TreeHead(treeSize int64) (*MapTreeHead, error) {
	contents, _, err := self.Client.MakeRequest("GET", fmt.Sprintf("/tree/%d", treeSize), nil, nil)
	if err != nil {
		return nil, err
	}
	var cr JSONMapTreeHeadResponse
	err = json.Unmarshal(contents, &cr)
	if err != nil {
		return nil, err
	}
	return &MapTreeHead{
		RootHash: cr.MapHash,
		MutationLogTreeHead: LogTreeHead{
			TreeSize: cr.LogTreeHead.TreeSize,
			RootHash: cr.LogTreeHead.Hash,
		},
	}, nil
}