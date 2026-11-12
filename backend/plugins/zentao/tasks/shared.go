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
	"fmt"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/zentao/models"
)

type input struct {
	Id int64
}

func GetTotalPagesFromResponse(res *http.Response, args *api.ApiCollectorArgs) (int, errors.Error) {
	body := &ZentaoPagination{}
	err := api.UnmarshalResponse(res, body)
	if err != nil {
		return 0, err
	}
	pages := body.Total / args.PageSize
	if body.Total%args.PageSize > 0 {
		pages++
	}
	return pages, nil
}

func getAccountId(account *models.ZentaoAccount) int64 {
	if account != nil {
		return account.ID
	}
	return 0
}

// get the Priority string for zentao
func getPriority(pri int) string {
	return fmt.Sprintf("%d", pri)
	/*
		switch pri {
		case 2:
			return "High"
		case 3:
			return "Middle"
		case 4:
			return "Low"
		default:
			if pri <= 1 {
				return "VeryHigh"
			}
			if pri >= 5 {
				return "VeryLow"
			}
		}
		return "Error"
	*/
}

// getSeverity formats the numeric zentao bug severity (e.g. 3) as its decimal string ("3")
func getSeverity(severity int) string {
	return strconv.Itoa(severity)
}

func getOriginalProject(data *ZentaoTaskData) string {
	if data.Options.ProjectId != 0 {
		return data.ProjectName
	}
	return ""
}

// getBugStatusMapping creates a map of original status values to bug issue standard status values
// based on the provided ZentaoTaskData. It returns the created map.
func getBugStatusMapping(data *ZentaoTaskData) map[string]string {
	stdStatusMappings := make(map[string]string)
	if data.Options.ScopeConfig == nil {
		return stdStatusMappings
	}
	mapping := data.Options.ScopeConfig.BugStatusMappings
	// Map original status values to standard status values
	for userStatus, stdStatus := range mapping {
		stdStatusMappings[userStatus] = strings.ToUpper(stdStatus)
	}
	return stdStatusMappings
}

// getStoryStatusMapping creates a map of original status values to story issue standard status values
// based on the provided ZentaoTaskData. It returns the created map.
func getStoryStatusMapping(data *ZentaoTaskData) map[string]string {
	stdStatusMappings := make(map[string]string)
	if data.Options.ScopeConfig == nil {
		return stdStatusMappings
	}
	mapping := data.Options.ScopeConfig.StoryStatusMappings
	// Map original status values to standard status values
	for userStatus, stdStatus := range mapping {
		stdStatusMappings[userStatus] = strings.ToUpper(stdStatus)
	}
	return stdStatusMappings
}

// getTaskStatusMapping creates a map of original status values to task issue standard status values
// based on the provided ZentaoTaskData. It returns the created map.
func getTaskStatusMapping(data *ZentaoTaskData) map[string]string {
	stdStatusMappings := make(map[string]string)
	if data.Options.ScopeConfig == nil {
		return stdStatusMappings
	}
	mapping := data.Options.ScopeConfig.TaskStatusMappings
	// Map original status values to standard status values
	for userStatus, stdStatus := range mapping {
		stdStatusMappings[userStatus] = strings.ToUpper(stdStatus)
	}
	return stdStatusMappings
}

// getStdTypeMappings creates a map of user type to standard type based on the provided ZentaoTaskData.
// It returns the created map.
func getStdTypeMappings(data *ZentaoTaskData) map[string]string {
	stdTypeMappings := make(map[string]string)
	if data.Options.ScopeConfig == nil {
		return stdTypeMappings
	}
	mapping := data.Options.ScopeConfig.TypeMappings
	// Map user types to standard types
	for userType, stdType := range mapping {
		stdTypeMappings[userType] = strings.ToUpper(stdType)
	}
	return stdTypeMappings
}

// parseRepoUrl parses a repository URL and returns the host, namespace, and repository name.
func parseRepoUrl(repoUrl string) (string, string, string, error) {
	parsedUrl, err := url.Parse(repoUrl)
	if err != nil {
		return "", "", "", err
	}

	host := parsedUrl.Hostname()
	host = strings.TrimPrefix(host, "www.")
	pathParts := strings.Split(parsedUrl.Path, "/")
	if len(pathParts) < 3 {
		return "", "", "", fmt.Errorf("invalid RepoUrl: %s", repoUrl)
	}

	namespace := strings.Join(pathParts[1:len(pathParts)-1], "/")
	repoName := pathParts[len(pathParts)-1]
	if repoName == "" {
		return "", "", "", fmt.Errorf("invalid RepoUrl: %s (empty repository name)", repoUrl)
	}

	return host, namespace, repoName, nil
}

func ignoreHTTPStatus404(res *http.Response) errors.Error {
	if res.StatusCode == http.StatusUnauthorized {
		return errors.Unauthorized.New("authentication failed, please check your AccessToken")
	}
	if res.StatusCode == http.StatusNotFound {
		return api.ErrIgnoreAndContinue
	}
	return nil
}

//func getProductIterator(taskCtx plugin.SubTaskContext) (dal.Rows, *api.DalCursorIterator, errors.Error) {
//	data := taskCtx.GetData().(*ZentaoTaskData)
//	db := taskCtx.GetDal()
//	clauses := []dal.Clause{
//		dal.Select("id"),
//		dal.From(&models.ZentaoProductSummary{}),
//		dal.Where(
//			"project_id = ? AND connection_id = ?",
//			data.Options.ProjectId, data.Options.ConnectionId,
//		),
//	}
//
//	cursor, err := db.Cursor(clauses...)
//	if err != nil {
//		return nil, nil, err
//	}
//	iterator, err := api.NewDalCursorIterator(db, cursor, reflect.TypeOf(input{}))
//	if err != nil {
//		cursor.Close()
//		return nil, nil, err
//	}
//	return cursor, iterator, nil
//}

func getExecutionIterator(taskCtx plugin.SubTaskContext) (dal.Rows, *api.DalCursorIterator, errors.Error) {
	data := taskCtx.GetData().(*ZentaoTaskData)
	db := taskCtx.GetDal()
	clauses := []dal.Clause{
		dal.Select("id"),
		dal.From(&models.ZentaoExecutionSummary{}),
		dal.Where(
			"project = ? AND connection_id = ?",
			data.Options.ProjectId, data.Options.ConnectionId,
		),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return nil, nil, err
	}
	iterator, err := api.NewDalCursorIterator(db, cursor, reflect.TypeOf(input{}))
	if err != nil {
		cursor.Close()
		return nil, nil, err
	}
	return cursor, iterator, nil
}

// AccountCache is a cache for account information.
type AccountCache struct {
	accounts     map[string]models.ZentaoAccount
	db           dal.Dal
	connectionId uint64
}

func NewAccountCache(db dal.Dal, connectionId uint64) *AccountCache {
	return &AccountCache{db: db, connectionId: connectionId, accounts: make(map[string]models.ZentaoAccount)}
}

func (a *AccountCache) put(account models.ZentaoAccount) {
	if account.Account != "" {
		a.accounts[account.Account] = account
	}
}

func (a *AccountCache) getAccountID(account string) int64 {
	if data, ok := a.accounts[account]; ok {
		return data.ID
	}
	var zentaoAccount models.ZentaoAccount
	err := a.db.First(
		&zentaoAccount,
		dal.Where("connection_id = ? AND account = ?", a.connectionId, account),
	)
	if err != nil {
		return 0
	}
	a.accounts[account] = zentaoAccount
	return zentaoAccount.ID
}

func (a *AccountCache) getAccountIDFromApiAccount(account *models.ApiAccount) int64 {
	if account == nil {
		return 0
	}
	if account.ID != 0 {
		return account.ID
	}
	return a.getAccountID(account.Account)
}

func (a *AccountCache) getAccountName(account string) string {
	if data, ok := a.accounts[account]; ok {
		return data.Realname
	}
	var zentaoAccount models.ZentaoAccount
	err := a.db.First(
		&zentaoAccount,
		dal.Where("connection_id = ? AND account = ?", a.connectionId, account),
	)
	if err != nil {
		return ""
	}
	a.accounts[account] = zentaoAccount
	return zentaoAccount.Realname
}

func (a *AccountCache) getAccountNameFromApiAccount(account *models.ApiAccount) string {
	if account == nil {
		return ""
	}
	if account.Realname != "" {
		return account.Realname
	}
	return a.getAccountName(account.Account)
}

func convertIssueURL(apiURL, issueType string, id int64) string {
	u, err := url.Parse(apiURL)
	if err != nil {
		return apiURL
	}
	before, _, _ := strings.Cut(u.Path, "/api.php/v1")
	u.RawQuery = ""
	u.Path = path.Join(before, fmt.Sprintf("/%s-view-%d.html", issueType, id))
	return u.String()
}

func extractIdFromLogComment(logCommentType string, comment string) ([]string, error) {
	if logCommentType != "task" && logCommentType != "bug" && logCommentType != "story" {
		return nil, errors.Default.New(fmt.Sprintf("unsupportted log comment type: %s", logCommentType))
	}
	regexpStr := fmt.Sprintf("(%s-view-\\d+\\.json)+", logCommentType)
	re := regexp.MustCompile(regexpStr)
	results := re.FindAllString(comment, -1)
	var ret []string

	convertMatchedString := func(s string) string {
		if s == "" {
			return s
		}
		s = strings.Replace(s, "-", " ", -1)
		s = strings.Replace(s, ".", " ", -1)
		return s
	}

	for _, matched := range results {
		var id string
		format := fmt.Sprintf("%s view %%s json", logCommentType)
		n, err := fmt.Sscanf(convertMatchedString(matched), format, &id)
		if err != nil {
			return nil, err
		}
		if n < 1 {
			return nil, errors.Default.New("unexpected comment")
		}
		ret = append(ret, id)
	}
	return ret, nil
}

// getZentaoHomePage receive endpoint like "http://54.158.1.10:30001/api.php/v1/" and return zentao's homepage like "http://54.158.1.10:30001/"
func getZentaoHomePage(endpoint string) (string, error) {
	if endpoint == "" {
		return "", errors.Default.New("empty endpoint")
	}
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	} else {
		protocol := endpointURL.Scheme
		host := endpointURL.Host
		zentaoPath, _, _ := strings.Cut(endpointURL.Path, "/api.php/v1")
		return fmt.Sprintf("%s://%s%s", protocol, host, zentaoPath), nil
	}
}
