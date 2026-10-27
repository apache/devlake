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

from redmine.api import RedmineAPI
from redmine.models import Connection, Project, ScopeConfig
from redmine.streams.issues import Issues
from redmine.streams.issue_journals import IssueJournals
from redmine.streams.time_entries import TimeEntries

from pydevlake import Plugin, RemoteScopeGroup, TestConnectionResult
from pydevlake.logger import logger
import pydevlake.domain_layer.ticket as ticket

ROOT_GROUP_ID = 'root'


class RedminePlugin(Plugin):

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
        yield ticket.Board(
            name=project.name,
            type='Scrum',
        )

    def remote_scope_groups(self, connection) -> list[RemoteScopeGroup]:
        return [RemoteScopeGroup(id=ROOT_GROUP_ID, name='Projects')]

    def remote_scopes(self, connection, group_id: str) -> list[Project]:
        with RedmineAPI(connection) as api:
            for raw in api.projects():
                yield Project(
                    id=str(raw['id']),
                    name=raw.get('name', ''),
                    identifier=raw.get('identifier'),
                    description=raw.get('description'),
                )

    def test_connection(self, connection: Connection) -> TestConnectionResult:
        with RedmineAPI(connection) as api:
            try:
                api.test_connection()
                return TestConnectionResult(success=True, message='Connection successful', status=200)
            except Exception as e:
                logger.warning("Redmine test_connection failed", exc_info=True)
                return TestConnectionResult(success=False, message=str(e), status=400)

    @property
    def streams(self):
        return [Issues, IssueJournals, TimeEntries]


if __name__ == '__main__':
    RedminePlugin.start()
