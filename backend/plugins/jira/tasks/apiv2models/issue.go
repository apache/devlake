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

package apiv2models

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/plugins/jira/models"
)

// FlexibleDescription supports both plain text and ADF (Atlassian Document Format) for Jira description field
// ADF reference: https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/
type FlexibleDescription struct {
	Value string
}

// ADFNode represents a node in Atlassian Document Format
type ADFNode struct {
	Type    string                 `json:"type"`
	Text    string                 `json:"text,omitempty"`
	Content []ADFNode              `json:"content,omitempty"`
	Attrs   map[string]interface{} `json:"attrs,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for FlexibleDescription
func (fd *FlexibleDescription) UnmarshalJSON(data []byte) error {
	// Handle null values
	if string(data) == "null" {
		fd.Value = ""
		return nil
	}

	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		fd.Value = str
		return nil
	}

	// Try to unmarshal as ADF document
	var adfDoc ADFNode
	if err := json.Unmarshal(data, &adfDoc); err == nil {
		fd.Value = extractTextFromADF(adfDoc)
		return nil
	}

	// Fallback: keep raw JSON as string for debugging
	fd.Value = string(data)
	return nil
}

// extractTextFromADF recursively extracts plain text from ADF document
func extractTextFromADF(node ADFNode) string {
	var result strings.Builder

	switch node.Type {
	case "text":
		result.WriteString(node.Text)
	case "hardBreak":
		result.WriteString("\n")
	case "paragraph":
		for _, child := range node.Content {
			result.WriteString(extractTextFromADF(child))
		}
		result.WriteString("\n")
	case "heading":
		for _, child := range node.Content {
			result.WriteString(extractTextFromADF(child))
		}
		result.WriteString("\n")
	case "listItem":
		for _, child := range node.Content {
			result.WriteString(extractTextFromADF(child))
		}
	case "bulletList", "orderedList":
		for _, child := range node.Content {
			result.WriteString("• ")
			result.WriteString(extractTextFromADF(child))
			result.WriteString("\n")
		}
	case "table":
		for _, row := range node.Content {
			if row.Type == "tableRow" {
				for j, cell := range row.Content {
					if j > 0 {
						result.WriteString(" | ")
					}
					result.WriteString(extractTextFromADF(cell))
				}
				result.WriteString("\n")
			}
		}
	case "tableCell", "tableHeader":
		for _, child := range node.Content {
			result.WriteString(extractTextFromADF(child))
		}
	case "codeBlock":
		result.WriteString("```\n")
		for _, child := range node.Content {
			result.WriteString(extractTextFromADF(child))
		}
		result.WriteString("\n```\n")
	case "blockquote":
		result.WriteString("> ")
		for _, child := range node.Content {
			result.WriteString(extractTextFromADF(child))
		}
		result.WriteString("\n")
	case "doc":
		for _, child := range node.Content {
			result.WriteString(extractTextFromADF(child))
		}
	case "inlineCard", "mention":
		// Extract text from attrs or content for links and mentions
		if attrs, ok := node.Attrs["text"]; ok {
			if text, ok := attrs.(string); ok {
				result.WriteString(text)
			}
		} else {
			for _, child := range node.Content {
				result.WriteString(extractTextFromADF(child))
			}
		}
	default:
		// For unknown types, extract content recursively
		for _, child := range node.Content {
			result.WriteString(extractTextFromADF(child))
		}
	}

	return result.String()
}

// String returns the string value
func (fd FlexibleDescription) String() string {
	return fd.Value
}

type Issue struct {
	Expand string `json:"expand"`
	ID     uint64 `json:"id,string"`
	Self   string `json:"self"`
	Key    string `json:"key"`
	Fields struct {
		AllFields map[string]interface{}
		Issuetype struct {
			Self        string `json:"self"`
			ID          string `json:"id"`
			Description string `json:"description"`
			IconURL     string `json:"iconUrl"`
			Name        string `json:"name"`
			Subtask     bool   `json:"subtask"`
			AvatarID    int    `json:"avatarId"`
		} `json:"issuetype"`
		Parent *struct {
			ID  uint64 `json:"id,string"`
			Key string `json:"key"`
		} `json:"parent"`
		Timespent     *int64   `json:"timespent"`
		Sprint        *Sprint  `json:"sprint"`
		ClosedSprints []Sprint `json:"closedSprints"`
		Project       struct {
			Self           string `json:"self"`
			ID             uint64 `json:"id,string"`
			Key            string `json:"key"`
			Name           string `json:"name"`
			ProjectTypeKey string `json:"projectTypeKey"`
			AvatarUrls     struct {
				Four8X48  string `json:"48x48"`
				Two4X24   string `json:"24x24"`
				One6X16   string `json:"16x16"`
				Three2X32 string `json:"32x32"`
			} `json:"avatarUrls"`
		} `json:"project"`
		FixVersions []struct {
			Self string `json:"self"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"fixVersions"`
		Aggregatetimespent interface{}         `json:"aggregatetimespent"`
		Resolution         interface{}         `json:"resolution"`
		Resolutiondate     *common.Iso8601Time `json:"resolutiondate"`
		Workratio          int                 `json:"workratio"`
		LastViewed         string              `json:"lastViewed"`
		Watches            struct {
			Self       string `json:"self"`
			WatchCount int    `json:"watchCount"`
			IsWatching bool   `json:"isWatching"`
		} `json:"watches"`
		Created *common.Iso8601Time `json:"created"`
		Epic    *struct {
			ID      int    `json:"id"`
			Key     string `json:"key"`
			Self    string `json:"self"`
			Name    string `json:"name"`
			Summary string `json:"summary"`
			Color   struct {
				Key string `json:"key"`
			} `json:"color"`
			Done bool `json:"done"`
		} `json:"epic"`
		Priority *struct {
			Self    string `json:"self"`
			IconURL string `json:"iconUrl"`
			Name    string `json:"name"`
			ID      uint64 `json:"id,string"`
		} `json:"priority"`
		Labels                        []string           `json:"labels"`
		Timeestimate                  interface{}        `json:"timeestimate"`
		Aggregatetimeoriginalestimate interface{}        `json:"aggregatetimeoriginalestimate"`
		Versions                      []interface{}      `json:"versions"`
		Issuelinks                    []IssueLink        `json:"issuelinks"`
		Assignee                      *Account           `json:"assignee"`
		Updated                       common.Iso8601Time `json:"updated"`
		Status                        struct {
			Self           string `json:"self"`
			Description    string `json:"description"`
			IconURL        string `json:"iconUrl"`
			Name           string `json:"name"`
			ID             string `json:"id"`
			StatusCategory struct {
				Self      string `json:"self"`
				ID        int    `json:"id"`
				Key       string `json:"key"`
				ColorName string `json:"colorName"`
				Name      string `json:"name"`
			} `json:"statusCategory"`
		} `json:"status"`
		Components []struct {
			Self string `json:"self"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"components"`
		Timeoriginalestimate *int64              `json:"timeoriginalestimate"`
		Description          FlexibleDescription `json:"description"`
		Timetracking         *struct {
			RemainingEstimate        string `json:"remainingEstimate"`
			TimeSpent                string `json:"timeSpent"`
			RemainingEstimateSeconds int64  `json:"remainingEstimateSeconds"`
			TimeSpentSeconds         int    `json:"timeSpentSeconds"`
		} `json:"timetracking"`
		Archiveddate          interface{}   `json:"archiveddate"`
		Aggregatetimeestimate *int64        `json:"aggregatetimeestimate"`
		Summary               string        `json:"summary"`
		Creator               Account       `json:"creator"`
		Subtasks              []interface{} `json:"subtasks"`
		Reporter              Account       `json:"reporter"`
		Aggregateprogress     struct {
			Progress int `json:"progress"`
			Total    int `json:"total"`
		} `json:"aggregateprogress"`
		Environment interface{} `json:"environment"`
		Duedate     string      `json:"duedate"` // yyyy-MM-dd
		Progress    struct {
			Progress int `json:"progress"`
			Total    int `json:"total"`
		} `json:"progress"`
		Worklog *struct {
			StartAt    int       `json:"startAt"`
			MaxResults int       `json:"maxResults"`
			Total      int       `json:"total"`
			Worklogs   []Worklog `json:"worklogs"`
		} `json:"worklog"`
		Comment struct {
			Comments   []Comment `json:"comments"`
			Self       string    `json:"self"`
			MaxResults int       `json:"maxResults"`
			Total      int       `json:"total"`
			StartAt    int       `json:"startAt"`
		} `json:"comment"`
	} `json:"fields"`
	Changelog *struct {
		StartAt    int         `json:"startAt"`
		MaxResults int         `json:"maxResults"`
		Total      int         `json:"total"`
		Histories  []Changelog `json:"histories"`
	} `json:"changelog"`
}

type IssueLinkType struct {
	ID      uint64 `json:"id,string"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
	Self    string `json:"self"`
}

type InOutwardIssue struct {
	ID     uint64 `json:"id,string"`
	Key    string `json:"key"`
	Self   string `json:"self"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Self           string `json:"self"`
			Description    string `json:"description"`
			IconURL        string `json:"iconUrl"`
			Name           string `json:"name"`
			ID             uint64 `json:"id,string"`
			StatusCategory struct {
				Self      string `json:"self"`
				ID        int    `json:"id"`
				Key       string `json:"key"`
				ColorName string `json:"colorName"`
				Name      string `json:"name"`
			} `json:"statusCategory"`
		} `json:"status"`
		Priority struct {
			Self    string `json:"self"`
			IconURL string `json:"iconUrl"`
			Name    string `json:"name"`
			ID      uint64 `json:"id,string"`
		} `json:"priority"`
		Issuetype struct {
			Self        string `json:"self"`
			ID          uint64 `json:"id,string"`
			Description string `json:"description"`
			IconURL     string `json:"iconUrl"`
			Name        string `json:"name"`
			Subtask     bool   `json:"subtask"`
			AvatarID    int    `json:"avatarId"`
		} `json:"issuetype"`
	} `json:"fields"`
}

type IssueLink struct {
	ID           uint64         `json:"id,string"`
	Self         string         `json:"self"`
	Type         IssueLinkType  `json:"type"`
	InwardIssue  InOutwardIssue `json:"inwardIssue"`
	OutwardIssue InOutwardIssue `json:"outwardIssue"`
}

func (i Issue) toToolLayer(connectionId uint64) *models.JiraIssue {
	var workload float64
	result := &models.JiraIssue{
		ConnectionId:       connectionId,
		IssueId:            i.ID,
		ProjectId:          i.Fields.Project.ID,
		ProjectName:        i.Fields.Project.Name,
		Self:               i.Self,
		IconURL:            i.Fields.Issuetype.IconURL,
		IssueKey:           i.Key,
		StoryPoint:         &workload,
		Summary:            i.Fields.Summary,
		Description:        i.Fields.Description.Value,
		Type:               i.Fields.Issuetype.ID,
		StatusName:         i.Fields.Status.Name,
		StatusKey:          i.Fields.Status.StatusCategory.Key,
		ResolutionDate:     i.Fields.Resolutiondate.ToNullableTime(),
		CreatorAccountId:   i.Fields.Creator.getAccountId(),
		CreatorDisplayName: i.Fields.Creator.DisplayName,
		Created:            i.Fields.Created.ToTime(),
		Updated:            i.Fields.Updated.ToTime(),
	}
	if i.Changelog != nil {
		result.ChangelogTotal = i.Changelog.Total
	}
	if i.Fields.Worklog != nil {
		result.WorklogTotal = i.Fields.Worklog.Total
	}
	if i.Fields.Epic != nil {
		result.EpicKey = i.Fields.Epic.Key
	}
	if i.Fields.Assignee != nil {
		result.AssigneeAccountId = i.Fields.Assignee.getAccountId()
		result.AssigneeDisplayName = i.Fields.Assignee.DisplayName
	}
	if i.Fields.Priority != nil {
		result.PriorityId = i.Fields.Priority.ID
		result.PriorityName = i.Fields.Priority.Name
	}
	if i.Fields.Timeoriginalestimate != nil {
		temp := *i.Fields.Timeoriginalestimate / 60
		result.OriginalEstimateMinutes = &temp
	}
	if i.Fields.Aggregatetimeestimate != nil {
		result.AggregateEstimateMinutes = *i.Fields.Aggregatetimeestimate / 60
	}
	if i.Fields.Timetracking != nil {
		result.RemainingEstimateMinutes = i.Fields.Timetracking.RemainingEstimateSeconds / 60
	}
	if i.Fields.Parent != nil {
		result.ParentId = i.Fields.Parent.ID
		result.ParentKey = i.Fields.Parent.Key
	}
	if i.Fields.Sprint != nil {
		result.SprintId = i.Fields.Sprint.ID
		result.SprintName = i.Fields.Sprint.Name
	}
	if i.Fields.Timespent != nil {
		temp := *i.Fields.Timespent / 60
		result.SpentMinutes = &temp
	}
	return result
}

func (i *Issue) SetAllFields(raw json.RawMessage) errors.Error {
	var issue2 struct {
		Expand string          `json:"expand"`
		ID     uint64          `json:"id,string"`
		Self   string          `json:"self"`
		Key    string          `json:"key"`
		Fields json.RawMessage `json:"fields"`
	}
	err := errors.Convert(json.Unmarshal(raw, &issue2))
	if err != nil {
		return err
	}
	err = errors.Convert(json.Unmarshal(issue2.Fields, &i.Fields.AllFields))
	if err != nil {
		return err
	}
	return nil
}

func (i Issue) ExtractEntities(connectionId uint64, userFieldMaps map[string]struct{}) ([]uint64, *models.JiraIssue, []*models.JiraIssueComment, []*models.JiraWorklog, []*models.JiraIssueChangelogs, []*models.JiraIssueChangelogItems, []*models.JiraAccount) {
	issue := i.toToolLayer(connectionId)
	var comments []*models.JiraIssueComment
	var worklogs []*models.JiraWorklog
	var changelogs []*models.JiraIssueChangelogs
	var changelogItems []*models.JiraIssueChangelogItems
	var users []*models.JiraAccount
	var sprints []uint64

	if i.Fields.Comment.Total > 0 {
		issue.CommentTotal = int64(i.Fields.Comment.Total)
		var issueUpdated *time.Time
		if len(i.Fields.Comment.Comments) <= i.Fields.Comment.Total {
			issueUpdated = i.Fields.Updated.ToNullableTime()
		}
		for _, c := range i.Fields.Comment.Comments {
			comments = append(comments, c.ToToolLayer(connectionId, i.ID, issueUpdated))
		}
	}
	if i.Fields.Worklog != nil {
		var issueUpdated *time.Time
		if len(i.Fields.Worklog.Worklogs) <= i.Fields.Worklog.Total {
			issueUpdated = i.Fields.Updated.ToNullableTime()
		}
		for _, w := range i.Fields.Worklog.Worklogs {
			worklogs = append(worklogs, w.ToToolLayer(connectionId, issueUpdated))
		}
	}
	if i.Changelog != nil {
		var issueUpdated *time.Time
		if len(i.Changelog.Histories) < 100 {
			issueUpdated = i.Fields.Updated.ToNullableTime()
		}
		for _, changelog := range i.Changelog.Histories {
			cl, user := changelog.ToToolLayer(connectionId, i.ID, issueUpdated)
			changelogs = append(changelogs, cl)
			if user != nil {
				users = append(users, user)
			}
			for _, indexed := range IndexChangelogItems(changelog.Items) {
				changelogItems = append(changelogItems, indexed.Item.ToToolLayer(connectionId, changelog.ID, indexed.ItemIndex))
				users = append(users, indexed.Item.ExtractUser(connectionId, userFieldMaps)...)
			}
		}
	}
	if i.Fields.Sprint != nil {
		sprints = append(sprints, i.Fields.Sprint.ID)
	}
	for _, sprint := range i.Fields.ClosedSprints {
		sprints = append(sprints, sprint.ID)
	}
	if creator := i.Fields.Creator.ToToolLayer(connectionId); creator != nil {
		users = append(users, creator)
	}
	if reporter := i.Fields.Reporter.ToToolLayer(connectionId); reporter != nil {
		users = append(users, reporter)
	}
	if i.Fields.Assignee != nil {
		if assignee := i.Fields.Assignee.ToToolLayer(connectionId); assignee != nil {
			users = append(users, assignee)
		}
	}
	return sprints, issue, comments, worklogs, changelogs, changelogItems, users
}
