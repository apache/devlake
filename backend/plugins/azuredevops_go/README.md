<!--
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
-->

# Azure DevOps Go Plugin

This plugin can coexist with the Python Azure DevOps plugin at `../../python/plugins/azuredevops`; the two implementations use separate `_raw` and `_tool` tables.

**Read access** to the following Azure DevOps Scopes is required:

- Build
- Code
- Graph (collectAccounts task)
- Release

Discovering external repositories also requires permission to read Service Connections.
Cloud Graph account collection is skipped for on-premises connections.
Source-provider repository HTTP errors are logged with their status and up to
4 KiB of response detail. A failing service connection is skipped so repositories
from other service connections can still be discovered.

## Azure DevOps Server (On-Premises)

Select **Azure DevOps Server (On-Premises)** and enter the full collection URL
(for example, `https://your-server/tfs/DefaultCollection/`). Username is optional
when authenticating with a PAT. Editing a connection preserves its server URL.

The Go plugin automatically probes REST API versions **7.2, 7.1, 7.0, and 6.0**
in descending order when initializing an API client, including during **Test
Connection**. Only HTTP 400 with `VssVersionNotSupportedException` or
`VssVersionOutOfRangeException` triggers a lower version; authentication,
permission, and connectivity failures are reported.
The selected version is reused for scope discovery and collection requests made
by that client. It is detected again for new clients, so changing the endpoint
or upgrading the server requires no manual version setting or database migration.
Servers must support REST API 6.0 or later (Azure DevOps Server 2020 or newer).
Cloud connections retain their existing API versions.

### Endpoint compatibility

The server API family and the resource's preview revision are separate. Requests
use the following versions from the Microsoft REST reference:

| Endpoint | 6.0 | 7.0 | 7.1 | 7.2 |
| --- | --- | --- | --- | --- |
| Projects | 6.0 | 7.0 | 7.1 | 7.2-preview.4 |
| Git repositories, commits, pull request list | 6.0 | 7.0 | 7.1 | 7.2-preview.2 |
| Pull request commits | 6.0 | 7.0 | 7.1 | 7.2-preview.1 |
| Builds | 6.0 | 7.0 | 7.1 | 7.2-preview.8 |
| Build timeline | 6.0 | 7.0 | 7.1 | 7.2-preview.3 |
| Service endpoints | 6.0-preview.4 | 7.0 | 7.1 | 7.2-preview.4 |
| Source provider repositories | 6.0-preview.1 | 7.0 | 7.1 | 7.2-preview.1 |

On-premises requests are relative to the collection URL, without a Cloud
organization prefix. Previously saved organization-prefixed scope IDs are
normalized to project-only parent IDs and empty organization IDs during
on-premises discovery. Cloud scope IDs retain their organization prefix.
External repository discovery follows the response body's `continuationToken`.
The timeline API is not paginated; pull request commits use continuation tokens,
not `$skip`.

REST 6.0 and 7.0 do not support the pull request `minTime`, `maxTime`, or
`queryTimeRangeType` search filters. Those parameters are omitted on these
servers, and all pull request pages are collected instead. This preserves data
collection but can take longer than the server-side date filtering used with
7.1 and 7.2. Build date filters remain supported on all four versions.

References: [6.0 service endpoints](https://learn.microsoft.com/en-us/rest/api/azure/devops/serviceendpoint/endpoints/get-service-endpoints?view=azure-devops-rest-6.0),
[7.2 projects](https://learn.microsoft.com/en-us/rest/api/azure/devops/core/projects/list?view=azure-devops-rest-7.2),
[7.2 builds](https://learn.microsoft.com/en-us/rest/api/azure/devops/build/builds/list?view=azure-devops-rest-7.2),
[7.0 pull requests](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/get-pull-requests?view=azure-devops-rest-7.0),
[7.1 pull requests](https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/get-pull-requests?view=azure-devops-rest-7.1),
and [source provider pagination](https://learn.microsoft.com/en-us/rest/api/azure/devops/build/source-providers/list-repositories?view=azure-devops-rest-6.0).