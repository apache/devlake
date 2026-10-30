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

# Azure Devops Python Plugin

This is a revamped version of the Python Azure DevOps Plugin, originally located at `../../python/plugins/azuredevops`.
The plugin is able to coexist with the Python version as both implementations come with their own `_raw` and `_tool` tables.

**Read access** to the following Azure DevOps Scopes is required:

- Build
- Code
- Graph (collectAccounts task)
- Release
- Work Items, to collect Azure Boards work items, iterations, comments, and changelogs
- Test Management, to collect test plans, test cases, test runs, and test results

Selecting a repository with work item or test entities collects that repository's project once. Further repositories of the same project in a blueprint do not collect the project again, and the work items and tests are stored on one board. Repository scope names stay the repository name. A project with no enabled repository lists one scope named after the project with a "(project)" suffix, and that scope collects its work items and tests.

Work items are stored in the ticket domain. The board is named after the project, uses the project URL, and has type Scrum. Custom fields are stored under both the Azure reference name and the display name. Test plans, cases, and executions are stored in the QA domain, including each test case's creation time and creator. The scope config can map work item types and states, and can turn comment and changelog collection on.

Access to Service Connections has been removed as they usually contain sensitive security information.