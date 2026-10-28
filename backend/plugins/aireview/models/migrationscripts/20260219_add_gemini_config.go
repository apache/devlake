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

package migrationscripts

import (
	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addGeminiConfig)(nil)

type addGeminiConfig struct{}

func (script *addGeminiConfig) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Add Gemini config columns to _tool_aireview_scope_configs
	err := db.AutoMigrate(&scopeConfigAddGemini20260219{})
	if err != nil {
		return errors.Default.Wrap(err, "failed to add Gemini columns to _tool_aireview_scope_configs")
	}

	return nil
}

func (script *addGeminiConfig) Version() uint64 {
	return 20260219000001
}

func (script *addGeminiConfig) Name() string {
	return "aireview add Gemini Code Assist configuration"
}

// scopeConfigAddGemini20260219 represents the table structure with new Gemini columns
type scopeConfigAddGemini20260219 struct {
	GeminiEnabled  bool   `gorm:"type:boolean;default:true"`
	GeminiUsername string `gorm:"type:varchar(255)"`
	GeminiPattern  string `gorm:"type:varchar(500)"`
}

func (scopeConfigAddGemini20260219) TableName() string {
	return "_tool_aireview_scope_configs"
}
