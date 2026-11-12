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

package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/devlake/helpers/pluginhelper/api/models"
	"github.com/apache/devlake/plugins/azuredevops_go/api/azuredevops"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"
)

const (
	idSeparator    = "/"
	maxConcurrency = 10
)

type AzuredevopsRemotePagination struct {
	Skip int
	Top  int
}

// https://learn.microsoft.com/en-us/azure/devops/pipelines/repos/?view=azure-devops
// https://learn.microsoft.com/en-us/azure/devops/pipelines/repos/multi-repo-checkout?view=azure-devops
var supportedSourceRepositories = []string{"github", "githubenterprise", "bitbucket", "git"}

func listAzuredevopsRemoteScopes(
	connection *models.AzuredevopsConnection,
	apiClient plugin.ApiClient,
	groupId string,
	page AzuredevopsRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AzuredevopsRepo],
	nextPage *AzuredevopsRemotePagination,
	err errors.Error,
) {

	org := connection.Organization
	// Use the connection endpoint for On-Premises, or VSSPS for Cloud
	clientUrl := "https://app.vssps.visualstudio.com"
	if connection.Endpoint != "" {
		clientUrl = connection.GetEndpoint()
	}
	vsc := azuredevops.NewClient(connection, apiClient, clientUrl)

	if groupId == "" {
		return listAzuredevopsProjects(connection, vsc, page, org)
	}

	id := strings.Split(groupId, idSeparator)
	orgId := id[0]
	projectId := ""
	if len(id) > 1 {
		projectId = id[1]
	} else if connection.Endpoint != "" {
		orgId = ""
		projectId = id[0]
	}

	if projectId != "" {
		if remote, listErr := listRemoteRepos(vsc, orgId, projectId); listErr == nil {
			children = append(children, remote...)
		} else {
			basicRes.GetLogger().Warn(listErr, "failed to list the external repositories of project %s", projectId)
		}

		if remote, listErr := listAzuredevopsRepos(vsc, orgId, projectId); listErr == nil {
			children = append(children, remote...)
		} else {
			basicRes.GetLogger().Warn(listErr, "failed to list the Azure DevOps repositories of project %s", projectId)
		}
	}
	return children, nextPage, nil
}

func listAzuredevopsProjects(connection *models.AzuredevopsConnection, vsc azuredevops.Client, _ AzuredevopsRemotePagination, org string) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AzuredevopsRepo],
	nextPage *AzuredevopsRemotePagination,
	err errors.Error) {

	var accounts azuredevops.AccountResponse
	if connection.Endpoint != "" {
		accounts = append(accounts, azuredevops.Account{AccountName: org})
	} else if org == "" {
		profile, err := vsc.GetUserProfile()
		if err != nil {
			return nil, nil, err
		}
		accounts, err = vsc.GetUserAccounts(profile.Id)
		if err != nil {
			return nil, nil, err
		}
	} else {
		accounts = append(accounts, azuredevops.Account{AccountName: org})
	}

	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(maxConcurrency)

	var mu sync.Mutex

	for _, v := range accounts {
		accountName := v.AccountName
		g.Go(func() error {
			args := azuredevops.GetProjectsArgs{
				OrgId: accountName,
			}
			projects, err := vsc.GetProjects(args)
			if err != nil {
				return err
			}

			var tmp []dsmodels.DsRemoteApiScopeListEntry[models.AzuredevopsRepo]
			for _, vv := range projects {
				groupId := vv.Name
				if accountName != "" {
					groupId = accountName + idSeparator + vv.Name
				}
				tmp = append(tmp, dsmodels.DsRemoteApiScopeListEntry[models.AzuredevopsRepo]{
					Id:   groupId,
					Type: api.RAS_ENTRY_TYPE_GROUP,
					Name: vv.Name,
				})
			}
			mu.Lock()
			children = append(children, tmp...)
			mu.Unlock()
			return nil
		})
	}

	err = errors.Convert(g.Wait())
	return
}

func listAzuredevopsRepos(
	vsc azuredevops.Client,
	orgId, projectId string,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AzuredevopsRepo],
	err errors.Error) {

	args := azuredevops.GetRepositoriesArgs{
		OrgId:     orgId,
		ProjectId: projectId,
	}

	repos, err := vsc.GetRepositories(args)
	if err != nil {
		return nil, err
	}

	for _, v := range repos {
		if v.IsDisabled {
			continue
		}

		pID := projectId
		if orgId != "" {
			pID = orgId + idSeparator + projectId
		}
		repo := models.AzuredevopsRepo{
			Id:        v.Id,
			Type:      models.RepositoryTypeADO,
			Name:      v.Name,
			Url:       v.Url,
			RemoteUrl: v.RemoteUrl,
			IsFork:    false,
		}
		repo.ProjectId = projectId
		repo.OrganizationId = orgId
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.AzuredevopsRepo]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			ParentId: &pID,
			Id:       v.Id,
			Name:     v.Name,
			FullName: v.Name,
			Data:     &repo,
		})
	}
	return
}

func listRemoteRepos(
	vsc azuredevops.Client,
	orgId, projectId string,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.AzuredevopsRepo],
	err errors.Error) {

	args := azuredevops.GetServiceEndpointsArgs{
		OrgId:     orgId,
		ProjectId: projectId,
	}

	endpoints, err := vsc.GetServiceEndpoints(args)
	if err != nil {
		return nil, err
	}

	var mu sync.Mutex
	var remoteRepos []azuredevops.RemoteRepository

	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(maxConcurrency)

	for _, v := range endpoints {
		if !slices.Contains(supportedSourceRepositories, v.Type) {
			continue
		}

		remoteRepoArgs := azuredevops.GetRemoteRepositoriesArgs{
			ProjectId:       projectId,
			OrgId:           orgId,
			Provider:        v.Type,
			ServiceEndpoint: v.Id,
		}
		endpointName := v.Name

		g.Go(func() error {
			repos, err := vsc.GetRemoteRepositories(remoteRepoArgs)
			if err != nil {
				// A single service connection that cannot be queried (e.g. expired credentials) must not
				// hide the repositories of all the other service connections, so skip it and keep going.
				basicRes.GetLogger().Warn(err, "failed to list the repositories of service connection %q (%s), skipping it", endpointName, remoteRepoArgs.Provider)
				return nil
			}
			mu.Lock()
			remoteRepos = append(remoteRepos, repos...)
			mu.Unlock()
			return nil
		})
	}

	// The goroutines never return an error, failures are logged and skipped above.
	_ = g.Wait()

	for _, v := range remoteRepos {
		pID := orgId + idSeparator + projectId
		isFork, _ := strconv.ParseBool(v.Properties.IsFork)
		isPrivate, _ := strconv.ParseBool(v.Properties.IsPrivate)

		// IDs must not contain URL reserved characters (e.g., "/"), as this breaks the routing in the scope API.
		// Accessing /plugins/azuredevops_go/connections/<id>/apache/devlake results in a 404 error, where
		// "apache/devlake" is the repository ID returned by ADOs sourceProviders API.
		// Therefore, we are creating our own ID, by combining the Service Connection and the External ID
		remoteId := fmt.Sprintf("%s-%s", v.Properties.ConnectedServiceId, v.Properties.ExternalId)

		repo := models.AzuredevopsRepo{
			Id:         remoteId,
			Type:       v.SourceProviderName,
			Name:       v.SourceProviderName + idSeparator + v.FullName,
			Url:        v.Properties.ManageUrl,
			RemoteUrl:  v.Properties.CloneUrl,
			ExternalId: v.Id,
			IsFork:     isFork,
			IsPrivate:  isPrivate,
		}

		repo.ProjectId = projectId
		repo.OrganizationId = orgId
		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.AzuredevopsRepo]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			ParentId: &pID,
			Id:       v.Id,
			Name:     v.Name,
			FullName: v.FullName,
			Data:     &repo,
		})
	}
	return
}
