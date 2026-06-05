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

import "github.com/apache/incubator-devlake/core/plugin"

var CollectUserActivityMeta = plugin.SubTaskMeta{
	Name:             "collectUserActivity",
	EntryPoint:       CollectUserActivity,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect per-user daily engagement metrics from the Anthropic Analytics API",
}

var ExtractUserActivityMeta = plugin.SubTaskMeta{
	Name:             "extractUserActivity",
	EntryPoint:       ExtractUserActivity,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract per-user daily engagement metrics into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectUserActivityMeta},
}

var CollectActivitySummaryMeta = plugin.SubTaskMeta{
	Name:             "collectActivitySummary",
	EntryPoint:       CollectActivitySummary,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect organisation-level daily activity summaries from the Anthropic Analytics API",
}

var ExtractActivitySummaryMeta = plugin.SubTaskMeta{
	Name:             "extractActivitySummary",
	EntryPoint:       ExtractActivitySummary,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract organisation-level daily activity summaries into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectActivitySummaryMeta},
}

var CollectChatProjectsMeta = plugin.SubTaskMeta{
	Name:             "collectChatProjects",
	EntryPoint:       CollectChatProjects,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect per-project daily chat usage from the Anthropic Analytics API",
}

var ExtractChatProjectsMeta = plugin.SubTaskMeta{
	Name:             "extractChatProjects",
	EntryPoint:       ExtractChatProjects,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract per-project daily chat usage into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectChatProjectsMeta},
}

var CollectSkillUsageMeta = plugin.SubTaskMeta{
	Name:             "collectSkillUsage",
	EntryPoint:       CollectSkillUsage,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect per-skill daily usage from the Anthropic Analytics API",
}

var ExtractSkillUsageMeta = plugin.SubTaskMeta{
	Name:             "extractSkillUsage",
	EntryPoint:       ExtractSkillUsage,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract per-skill daily usage into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectSkillUsageMeta},
}

var CollectConnectorUsageMeta = plugin.SubTaskMeta{
	Name:             "collectConnectorUsage",
	EntryPoint:       CollectConnectorUsage,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect per-connector daily usage from the Anthropic Analytics API",
}

var ExtractConnectorUsageMeta = plugin.SubTaskMeta{
	Name:             "extractConnectorUsage",
	EntryPoint:       ExtractConnectorUsage,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract per-connector daily usage into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectConnectorUsageMeta},
}
