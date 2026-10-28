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

package parser

import (
	"net/url"
)

type GitExtractorTaskData struct {
	Options         *GitExtractorOptions
	ParsedURL       *url.URL
	GitRepo         RepoCollector
	SkipAllSubtasks bool // silently skip all tasks without raising errors
}

type GitExtractorApiParams struct {
	RepoId string `json:"repoId" mapstructure:"repoId"`
}

type GitExtractorOptions struct {
	GitExtractorApiParams `json:",inline" mapstructure:",squash"`
	Name                  string `json:"name" mapstructure:"name"`
	Url                   string `json:"url" mapstructure:"url"`
	User                  string `json:"user" mapstructure:"user"`
	Password              string `json:"password" mapstructure:"password"`
	PrivateKey            string `json:"privateKey" mapstructure:"privateKey"`
	Passphrase            string `json:"passphrase" mapstructure:"passphrase"`
	Proxy                 string `json:"proxy" mapstructure:"proxy"`
	UseGoGit              *bool  `json:"useGoGit" mapstructure:"useGoGit"`
	SkipCommitStat        *bool  `json:"skipCommitStat" mapstructure:"skipCommitStat" comment:"skip all commit stat including added/deleted lines and commit files as well"`
	SkipCommitFiles       *bool  `json:"skipCommitFiles" mapstructure:"skipCommitFiles"`
	NoShallowClone        bool   `json:"noShallowClone" mapstructure:"noShallowClone"`
	ForceFullClone        bool   `json:"forceFullClone" mapstructure:"forceFullClone"`
	ConnectionId          uint64 `json:"connectionId" mapstructure:"connectionId,omitempty"`
	PluginName            string `json:"pluginName" mapstructure:"pluginName,omitempty"`
	// Configured by upstream plugin (e.g., GitLab) to exclude file extensions from commit stats
	ExcludeFileExtensions []string `json:"excludeFileExtensions" mapstructure:"excludeFileExtensions"`
}
