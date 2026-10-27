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

from contextlib import contextmanager
from typing import Iterator
from unittest.mock import MagicMock, Mock, patch

from redmine.api import RedmineAPI, _ISSUE_SORT_UPDATED_ON


def _paged_mock(items, has_next=False):
    paged = MagicMock()
    paged.items = items
    paged.next_page_request = MagicMock() if has_next else None
    return paged


class PatchedRedmineAPI:
    _AUTO_MOCK_METHODS = frozenset({'get'})

    def __init__(self):
        object.__setattr__(self, '_api', RedmineAPI(MagicMock()))
        object.__setattr__(self, '_mocks', {})
        object.__setattr__(self, '_patchers', [])

    def _install_mock(self, name: str, mock: Mock) -> Mock:
        self._mocks[name] = mock
        patcher = patch.object(self._api, name, mock)
        patcher.start()
        self._patchers.append(patcher)
        return mock

    def __getattr__(self, name):
        if name in self._mocks:
            return self._mocks[name]
        if name in self._AUTO_MOCK_METHODS:
            return self._install_mock(name, MagicMock())
        return getattr(self._api, name)

    def __setattr__(self, name, value):
        if name.startswith('_'):
            object.__setattr__(self, name, value)
        elif isinstance(value, Mock):
            self._install_mock(name, value)
        else:
            setattr(self._api, name, value)

    def __enter__(self):
        return self

    def __exit__(self, *_):
        for patcher in reversed(self._patchers):
            patcher.stop()


@contextmanager
def _patched_api() -> Iterator[PatchedRedmineAPI]:
    with PatchedRedmineAPI() as api:
        yield api


def test_iter_issues_stops_after_limit_on_single_page():
    with _patched_api() as api:
        api.get.return_value = _paged_mock([{'id': i} for i in range(1, 6)])
        collected = list(api._iter_issues({'project_id': '1', 'status_id': '*'}, limit=3))

    assert [issue['id'] for issue in collected] == [1, 2, 3]
    api.get.assert_called_once()
    assert api.get.call_args.kwargs['limit'] == 3


def test_iter_issues_stops_after_limit_across_pages():
    with _patched_api() as api:
        api.get.side_effect = [
            _paged_mock([{'id': i} for i in range(1, 101)], has_next=True),
            _paged_mock([{'id': i} for i in range(101, 106)]),
        ]
        collected = list(api._iter_issues({'project_id': '1', 'status_id': '*'}, limit=105))

    assert len(collected) == 105
    assert api.get.call_count == 2
    assert api.get.call_args_list[0].kwargs['limit'] == 100
    assert api.get.call_args_list[1].kwargs == {
        'project_id': '1',
        'status_id': '*',
        'limit': 5,
        'offset': 100,
    }


def test_iter_issues_stops_at_updated_since():
    issues = [
        {'id': 1, 'updated_on': '2024-02-01T10:00:00Z'},
        {'id': 2, 'updated_on': '2024-01-01T10:00:00Z'},
        {'id': 3, 'updated_on': '2023-12-01T10:00:00Z'},
    ]

    with _patched_api() as api:
        api.get.return_value = _paged_mock(issues)
        collected = list(api.iter_issues(
            '1',
            updated_since='2024-01-15T00:00:00Z',
        ))

    assert [issue['id'] for issue in collected] == [1]
    api.get.assert_called_once()
    assert api.get.call_args.kwargs['sort'] == _ISSUE_SORT_UPDATED_ON


def test_iter_issues_incremental_uses_paginated_list():
    with _patched_api() as api:
        api.get.side_effect = [
            _paged_mock([{'id': 1, 'updated_on': '2024-02-01T10:00:00Z'}], has_next=True),
            _paged_mock([{'id': 2, 'updated_on': '2024-01-01T10:00:00Z'}]),
        ]
        collected = list(api.iter_issues('1', updated_since='2024-01-02T00:00:00Z'))

    assert [issue['id'] for issue in collected] == [1]
    assert api.get.call_count == 2


def test_issue_fetches_single_issue_with_journals():
    with _patched_api() as api:
        response = MagicMock()
        response.json = {
            'issue': {
                'id': 42,
                'journals': [{'id': 1, 'notes': 'done'}],
            },
        }
        api.get.return_value = response

        issue = api.issue(42, include_journals=True)

    assert issue['id'] == 42
    assert len(issue['journals']) == 1
    api.get.assert_called_once_with('issues/42.json', include='journals')
