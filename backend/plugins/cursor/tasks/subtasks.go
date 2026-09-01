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

import "github.com/apache/devlake/core/plugin"

var CollectUsageEventsMeta = plugin.SubTaskMeta{
	Name:             "collectUsageEvents",
	EntryPoint:       CollectUsageEvents,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect usage events from the Cursor Admin API",
}

var ExtractUsageEventsMeta = plugin.SubTaskMeta{
	Name:             "extractUsageEvents",
	EntryPoint:       ExtractUsageEvents,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract usage events into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectUsageEventsMeta},
}

var CollectUserSpendMeta = plugin.SubTaskMeta{
	Name:             "collectUserSpend",
	EntryPoint:       CollectUserSpend,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect per-user billing cycle spend from the Cursor Admin API",
}

var ExtractUserSpendMeta = plugin.SubTaskMeta{
	Name:             "extractUserSpend",
	EntryPoint:       ExtractUserSpend,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract per-user billing cycle spend into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectUserSpendMeta},
}

var CollectMembersMeta = plugin.SubTaskMeta{
	Name:             "collectMembers",
	EntryPoint:       CollectMembers,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect team member roster from the Cursor Admin API",
}

var ExtractMembersMeta = plugin.SubTaskMeta{
	Name:             "extractMembers",
	EntryPoint:       ExtractMembers,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract team member roster into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectMembersMeta},
}

var CollectDailyUsageMeta = plugin.SubTaskMeta{
	Name:             "collectDailyUsage",
	EntryPoint:       CollectDailyUsage,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect per-user per-day adoption metrics from the Cursor Admin API",
}

var ExtractDailyUsageMeta = plugin.SubTaskMeta{
	Name:             "extractDailyUsage",
	EntryPoint:       ExtractDailyUsage,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract per-user per-day adoption metrics into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectDailyUsageMeta},
}

var CollectAiCodeCommitsMeta = plugin.SubTaskMeta{
	Name:             "collectAiCodeCommits",
	EntryPoint:       CollectAiCodeCommits,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect per-commit AI line attribution from the Cursor Enterprise API (skipped for non-enterprise keys)",
}

var ExtractAiCodeCommitsMeta = plugin.SubTaskMeta{
	Name:             "extractAiCodeCommits",
	EntryPoint:       ExtractAiCodeCommits,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract per-commit AI line attribution into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectAiCodeCommitsMeta},
}

var CollectAiCodeChangesMeta = plugin.SubTaskMeta{
	Name:             "collectAiCodeChanges",
	EntryPoint:       CollectAiCodeChanges,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect granular accepted AI changes from the Cursor Enterprise API (skipped for non-enterprise keys)",
}

var ExtractAiCodeChangesMeta = plugin.SubTaskMeta{
	Name:             "extractAiCodeChanges",
	EntryPoint:       ExtractAiCodeChanges,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract granular accepted AI changes into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectAiCodeChangesMeta},
}

var CollectBugbotReviewsMeta = plugin.SubTaskMeta{
	Name:             "collectBugbotReviews",
	EntryPoint:       CollectBugbotReviews,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect BugBot review analytics from the Cursor Analytics API (skipped when bugbot-reviews is inaccessible)",
}

var ExtractBugbotReviewsMeta = plugin.SubTaskMeta{
	Name:             "extractBugbotReviews",
	EntryPoint:       ExtractBugbotReviews,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract BugBot reviews and findings into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectBugbotReviewsMeta},
}

var CollectConversationInsightsMeta = plugin.SubTaskMeta{
	Name:             "collectConversationInsights",
	EntryPoint:       CollectConversationInsights,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Collect Conversation Insights from the Cursor Analytics API (skipped for non-enterprise keys or when insights are disabled)",
}

var ExtractConversationInsightsMeta = plugin.SubTaskMeta{
	Name:             "extractConversationInsights",
	EntryPoint:       ExtractConversationInsights,
	EnabledByDefault: true,
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Description:      "Extract Conversation Insights into tool-layer tables",
	Dependencies:     []*plugin.SubTaskMeta{&CollectConversationInsightsMeta},
}
