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

from unittest.mock import MagicMock, patch

from redmine.main import ROOT_GROUP_ID, RedminePlugin
from redmine.models import Connection

SAMPLE_PROJECTS = [
    {'id': 1, 'name': 'Alpha', 'identifier': 'alpha'},
    {'id': 2, 'name': 'Beta', 'identifier': 'beta'},
]


def _connection():
    return Connection(id=1, name='test', endpoint='https://redmine.example.com', token='token')


@patch('redmine.main.RedmineAPI')
def test_remote_scopes_returns_all_projects(RedmineAPI):
    plugin = RedminePlugin()
    api = MagicMock()
    api.projects.return_value = SAMPLE_PROJECTS
    api.__enter__.return_value = api
    api.__exit__.return_value = False
    RedmineAPI.return_value = api

    projects = list(plugin.remote_scopes(_connection(), ROOT_GROUP_ID))

    assert [project.name for project in projects] == ['Alpha', 'Beta']


@patch('redmine.main.RedmineAPI')
def test_remote_scope_groups_returns_projects_group(RedmineAPI):
    plugin = RedminePlugin()
    groups = list(plugin.remote_scope_groups(_connection()))

    assert len(groups) == 1
    assert groups[0].id == ROOT_GROUP_ID
    assert groups[0].name == 'Projects'
