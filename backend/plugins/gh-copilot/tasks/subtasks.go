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

import (
	"github.com/apache/incubator-devlake/core/plugin"
)

var CollectOrgMetricsMeta = plugin.SubTaskMeta{
	Name:             "collectOrgMetrics",
	EntryPoint:       CollectOrgMetrics,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect GitHub Copilot organization usage metrics reports",
}

var CollectCopilotSeatAssignmentsMeta = plugin.SubTaskMeta{
	Name:             "collectCopilotSeatAssignments",
	EntryPoint:       CollectCopilotSeatAssignments,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect GitHub Copilot seat assignments",
}

var CollectEnterpriseMetricsMeta = plugin.SubTaskMeta{
	Name:             "collectEnterpriseMetrics",
	EntryPoint:       CollectEnterpriseMetrics,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect GitHub Copilot enterprise usage metrics reports",
}

var CollectUserMetricsMeta = plugin.SubTaskMeta{
	Name:             "collectUserMetrics",
	EntryPoint:       CollectUserMetrics,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect GitHub Copilot enterprise user-level usage metrics reports",
}

var CollectUserTeamsMeta = plugin.SubTaskMeta{
	Name:             "collectUserTeams",
	EntryPoint:       CollectUserTeams,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect GitHub Copilot user-team mappings from user-teams-1-day report",
}

var ExtractOrgMetricsMeta = plugin.SubTaskMeta{
	Name:             "extractOrgMetrics",
	EntryPoint:       ExtractOrgMetrics,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract Copilot org metrics into unified daily metrics tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectOrgMetricsMeta},
}

var ExtractSeatsMeta = plugin.SubTaskMeta{
	Name:             "extractSeats",
	EntryPoint:       ExtractSeats,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract Copilot seat assignments into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectCopilotSeatAssignmentsMeta},
}

var ExtractEnterpriseMetricsMeta = plugin.SubTaskMeta{
	Name:             "extractEnterpriseMetrics",
	EntryPoint:       ExtractEnterpriseMetrics,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract Copilot enterprise metrics into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectEnterpriseMetricsMeta},
}

var ExtractUserMetricsMeta = plugin.SubTaskMeta{
	Name:             "extractUserMetrics",
	EntryPoint:       ExtractUserMetrics,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract Copilot user metrics into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectUserMetricsMeta},
}

var ExtractUserTeamsMeta = plugin.SubTaskMeta{
	Name:             "extractUserTeams",
	EntryPoint:       ExtractUserTeams,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract Copilot user-team mappings into tool-layer table",
	Dependencies:     []*plugin.SubTaskMeta{&CollectUserTeamsMeta},
}
