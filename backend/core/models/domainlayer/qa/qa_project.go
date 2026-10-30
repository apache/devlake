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

package qa

import (
	"github.com/apache/devlake/core/models/domainlayer"
	plugin "github.com/apache/devlake/core/plugin"
)

var _ plugin.Scope = (*QaProject)(nil)

// QaProject represents a QA project in the domain layer
type QaProject struct {
	domainlayer.DomainEntityExtended
	Name string `gorm:"type:varchar(255);comment:Project name"`
}

func (QaProject) TableName() string {
	return "qa_projects"
}

func (r *QaProject) ScopeId() string {
	return r.Id
}

func (r *QaProject) ScopeName() string {
	return r.Name
}
