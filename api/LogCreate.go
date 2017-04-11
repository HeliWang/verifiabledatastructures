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
	"golang.org/x/net/context"

	"github.com/continusec/verifiabledatastructures/pb"
)

func (s *LocalService) LogCreate(ctx context.Context, req *pb.LogCreateRequest) (*pb.LogCreateResponse, error) {
	err := s.verifyAccessForLog(req.Log, pb.Permission_PERM_LOG_CREATE)
	if err != nil {
		return nil, err
	}
	if req.Log.LogType != pb.LogType_STRUCT_TYPE_LOG {
		return nil, ErrInvalidRequest
	}
	if len(req.Log.Name) == 0 {
		return nil, ErrInvalidRequest
	}
	promise, err := s.Mutator.QueueMutation(&pb.Mutation{
		LogCreate: req,
	})
	if err != nil {
		return nil, err
	}
	err = promise.WaitUntilDone()
	if err != nil {
		return nil, err
	}
	return &pb.LogCreateResponse{}, nil
}