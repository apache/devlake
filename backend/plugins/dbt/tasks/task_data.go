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

package tasks

type DbtOptions struct {
	ProjectPath   string `json:"projectPath"`
	ProjectName   string `json:"projectName"`
	ProjectTarget string `json:"projectTarget"`
	// clone from git to projectPath if projectGitURL is not empty
	ProjectGitURL  string                 `json:"projectGitURL"`
	ProjectVars    map[string]interface{} `json:"projectVars"`
	SelectedModels []string               `json:"selectedModels"`
	FailFast       bool                   `json:"failFast"`
	ProfilesPath   string                 `json:"profilesPath"`
	Profile        string                 `json:"profile"`
	Threads        int                    `json:"threads"`
	NoVersionCheck bool                   `json:"noVersionCheck"`
	ExcludeModels  []string               `json:"excludeModels"`
	Selector       string                 `json:"selector"`
	State          string                 `json:"state"`
	Defer          bool                   `json:"defer"`
	NoDefer        bool                   `json:"noDefer"`
	FullRefresh    bool                   `json:"fullRefresh"`
	// deprecated, dbt run args
	Args  []string `json:"args"`
	Tasks []string `json:"tasks,omitempty"`

	ProjectBaseDir    string `json:"-"`
	ManagedProjectDir bool   `json:"-"`
}

type DbtTaskData struct {
	Options *DbtOptions
}
