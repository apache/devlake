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

package main

import (
	"github.com/apache/devlake/core/config"
	"github.com/apache/devlake/core/plugin"
	_ "github.com/apache/devlake/core/version"
	// Linking the generated registry compiles every plugin into this binary and
	// registers it at startup. Build with -tags devlake_external_plugins to
	// leave the registry empty and load plugins from PLUGIN_DIR instead.
	_ "github.com/apache/devlake/plugins/all"
	"github.com/apache/devlake/server/api"
)

func main() {
	v := config.GetConfig()
	encryptionSecret := v.GetString(plugin.EncodeKeyEnvStr)
	if encryptionSecret == "" {
		panic("ENCRYPTION_SECRET must be set in environment variable or .env file")
	}
	api.CreateAndRunApiServer()
}
