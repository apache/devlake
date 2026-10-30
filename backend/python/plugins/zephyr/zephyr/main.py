# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at

#     http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from zephyr.api import ZephyrAPI
from zephyr.models import Connection, Project, ScopeConfig
from zephyr.streams.folders import Folders
from zephyr.streams.test_case_issue_links import TestCaseIssueLinks
from zephyr.streams.test_case_labels import TestCaseLabels
from zephyr.streams.test_cases import TestCases
from zephyr.streams.test_cycles import TestCycles
from zephyr.streams.test_execution_issue_links import TestExecutionIssueLinks
from zephyr.streams.test_executions import TestExecutions

import pydevlake.domain_layer.qa as qa
from pydevlake import Plugin, RemoteScopeGroup, TestConnectionResult
from pydevlake.api import APIException
from pydevlake.logger import logger
from pydevlake.model import raw_data_params

ROOT_GROUP_ID = 'root'


class ZephyrPlugin(Plugin):

    @property
    def connection_type(self):
        return Connection

    @property
    def tool_scope_type(self):
        return Project

    @property
    def scope_config_type(self):
        return ScopeConfig

    def domain_scopes(self, project: Project):
        yield qa.QaProject(
            name=project.name or project.project_key or project.id,
        )

    def remote_scope_groups(self, connection) -> list[RemoteScopeGroup]:
        return [RemoteScopeGroup(id=ROOT_GROUP_ID, name='Jira Projects')]

    def remote_scopes(self, connection, group_id: str) -> list[Project]:
        raw_table = self._raw_scope_table_name()
        with ZephyrAPI(connection) as api:
            for raw in api.projects():
                if not raw.get('enabled', True):
                    continue
                project_key = raw.get('key', '')
                scope_id = project_key
                project = Project(
                    id=scope_id,
                    name=project_key,
                    jira_project_id=raw.get('jiraProjectId'),
                    project_key=project_key,
                )
                project.connection_id = connection.id
                project.raw_data_params = raw_data_params(connection.id, scope_id)
                project.raw_data_table = raw_table
                yield project

    def test_connection(self, connection: Connection) -> TestConnectionResult:
        with ZephyrAPI(connection) as api:
            try:
                api.test_connection()
                return TestConnectionResult(
                    success=True,
                    message='Connection successful',
                    status=200,
                )
            except APIException as e:
                logger.warning('Zephyr test_connection failed', exc_info=True)
                return TestConnectionResult(success=False, message=str(e), status=400)
            except Exception as e:
                logger.warning('Zephyr test_connection failed', exc_info=True)
                return TestConnectionResult(success=False, message=str(e), status=400)

    @property
    def streams(self):
        return [
            Folders,
            TestCases,
            TestCaseLabels,
            TestCaseIssueLinks,
            TestCycles,
            TestExecutions,
            TestExecutionIssueLinks,
        ]


if __name__ == '__main__':
    ZephyrPlugin.start()
