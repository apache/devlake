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

import glpi.main as glpi_main
from pydevlake.message import RemoteScope

from glpi.main import GlpiPlugin, ROOT_GROUP_ID
from tests.common import collect_context, glpi_api_mock


def test_remote_scopes_returns_all_entities_sorted():
    plugin = GlpiPlugin()
    with glpi_api_mock(glpi_main) as api:
        api.entities.return_value = [
            {'id': 2, 'name': 'Y', 'completename': 'Root entity > X > Y', 'entities_id': 1},
            {'id': 0, 'name': 'Root entity', 'completename': 'Root entity', 'entities_id': 0},
            {'id': 1, 'name': 'X', 'completename': 'Root entity > X', 'entities_id': 0},
        ]
        entities = list(plugin.remote_scopes(collect_context().connection, ROOT_GROUP_ID))

    assert [entity.completename for entity in entities] == [
        'Root entity',
        'Root entity > X',
        'Root entity > X > Y',
    ]


def test_make_remote_scopes_under_root_group():
    plugin = GlpiPlugin()
    with glpi_api_mock(glpi_main) as api:
        api.entities.return_value = [
            {'id': 0, 'name': 'Root entity', 'completename': 'Root entity', 'entities_id': 0},
            {'id': 1, 'name': 'X', 'completename': 'Root entity > X', 'entities_id': 0},
        ]
        scopes = list(plugin.make_remote_scopes(collect_context().connection, ROOT_GROUP_ID))

    assert len(scopes) == 2
    assert all(isinstance(scope, RemoteScope) for scope in scopes)
    assert all(scope.parent_id == ROOT_GROUP_ID for scope in scopes)
    assert [scope.name for scope in scopes] == ['Root entity', 'X']
