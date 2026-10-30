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

from zephyr.api import ZephyrAPI
from zephyr.models import Connection


def _connection():
    return Connection(
        id=1,
        name='test',
        endpoint='https://api.zephyrscale.smartbear.com/v2',
        token='token',
    )


def test_iter_nextgen_paginates():
    api = ZephyrAPI(_connection())
    page1 = MagicMock()
    page1.json = {
        'values': [{'id': 1}],
        'nextStartAtId': 10,
    }
    page2 = MagicMock()
    page2.json = {
        'values': [{'id': 2}],
        'nextStartAtId': 10,
    }
    with patch.object(api, 'get', side_effect=[page1, page2]) as get_mock:
        items = list(api._iter_nextgen('testcases/nextgen', projectKey='ABC'))
    assert [item['id'] for item in items] == [1, 2]
    assert get_mock.call_count == 2
    second_call = get_mock.call_args_list[1]
    assert second_call.kwargs['startAtId'] == 10


def test_project_lookups_index_by_id():
    api = ZephyrAPI(_connection())
    with patch.object(api, 'folders', return_value=[{'id': 5, 'name': 'Folder A'}]), patch.object(
        api, 'priorities', return_value=[]
    ), patch.object(api, 'statuses', return_value=[]):
        lookups = api.project_lookups('ABC')
    assert lookups.folders[5]['name'] == 'Folder A'
    assert lookups.components == {}
