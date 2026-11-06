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

package models

import (
	"encoding/json"
	"fmt"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/codequality"
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/models/domainlayer/qa"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
)

func newScopeByTypeName(typeName string) (plugin.Scope, errors.Error) {
	switch typeName {
	case "Repo":
		return &code.Repo{}, nil
	case "CqProject":
		return &codequality.CqProject{}, nil
	case "CicdScope":
		return &devops.CicdScope{}, nil
	case "Board":
		return &ticket.Board{}, nil
	case "QaProject":
		return &qa.QaProject{}, nil
	default:
		return nil, errors.BadInput.New(fmt.Sprintf("Unknown scope type %s", typeName))
	}
}

func (d DynamicDomainScope) Load() (plugin.Scope, errors.Error) {
	scope, type_err := newScopeByTypeName(d.TypeName)
	if type_err != nil {
		return nil, type_err
	}
	err := errors.Convert(json.Unmarshal([]byte(d.Data), &scope))
	if err != nil {
		return nil, err
	}
	return scope, nil
}
