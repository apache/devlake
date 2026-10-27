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

import datetime
from unittest.mock import MagicMock, patch

from pydevlake.testing import ContextBuilder

from redmine.main import RedminePlugin
from redmine.streams.issue_journals import IssueJournals
from redmine.streams.issues import Issues
from redmine.streams.time_entries import TimeEntries


def _collect_context(**scope_config_kwargs):
    builder = (
        ContextBuilder(RedminePlugin())
        .with_connection(endpoint='http://redmine.example.com', token='test_api_key')
        .with_scope('1', name='Test Project')
        .with_scope_config(**scope_config_kwargs)
    )
    return builder.build()


@patch('redmine.streams.issues.RedmineAPI')
def test_collect_uses_project_time_after_when_not_incremental(RedmineAPI):
    context = (
        ContextBuilder(RedminePlugin())
        .with_connection(endpoint='http://redmine.example.com', token='test_api_key')
        .with_scope('1', name='Test Project')
        .with_time_after(datetime.datetime(2024, 1, 1, tzinfo=datetime.timezone.utc))
        .build()
    )
    api = MagicMock()
    api.issues.return_value = iter([])
    api.base_url = 'http://redmine.example.com'
    api.__enter__.return_value = api
    api.__exit__.return_value = False
    RedmineAPI.return_value = api

    list(Issues(RedminePlugin().name).collect({}, context))

    api.issues.assert_called_once_with(
        '1',
        created_on='>=2024-01-01',
        updated_since=None,
        include_relations=True,
    )


@patch('redmine.streams.issues.RedmineAPI')
def test_collect_without_time_after_omits_filter(RedmineAPI):
    context = _collect_context()
    api = MagicMock()
    api.issues.return_value = iter([])
    api.base_url = 'http://redmine.example.com'
    api.__enter__.return_value = api
    api.__exit__.return_value = False
    RedmineAPI.return_value = api

    list(Issues(RedminePlugin().name).collect({}, context))

    api.issues.assert_called_once_with(
        '1',
        created_on=None,
        updated_since=None,
        include_relations=True,
    )


@patch('redmine.streams.issues.RedmineAPI')
def test_collect_uses_updated_since_when_incremental(RedmineAPI):
    context = _collect_context()
    context.options['incremental'] = True
    api = MagicMock()
    api.issues.return_value = iter([
        {'id': 1, 'updated_on': '2024-06-02T10:00:00Z'},
    ])
    api.base_url = 'http://redmine.example.com'
    api.__enter__.return_value = api
    api.__exit__.return_value = False
    RedmineAPI.return_value = api

    _, state = next(Issues(RedminePlugin().name).collect(
        {'updated_since': '2024-06-01T00:00:00Z'},
        context,
    ))

    api.issues.assert_called_once_with(
        '1',
        created_on=None,
        updated_since='2024-06-01T00:00:00Z',
        include_relations=True,
    )
    assert state['updated_since'] == '2024-06-02T10:00:00Z'


@patch('redmine.streams.time_entries.RedmineAPI')
def test_collect_time_entries_skips_when_disabled(RedmineAPI):
    context = _collect_context()
    api = MagicMock()
    api.time_entries.return_value = iter([{'id': 1}])
    api.base_url = 'http://redmine.example.com'
    api.__enter__.return_value = api
    api.__exit__.return_value = False
    RedmineAPI.return_value = api

    collected = list(TimeEntries(RedminePlugin().name).collect({}, context))

    assert collected == []
    RedmineAPI.assert_not_called()


@patch('redmine.streams.time_entries.RedmineAPI')
def test_collect_time_entries_fetches_when_enabled(RedmineAPI):
    context = _collect_context(collect_time_entries=True)
    api = MagicMock()
    api.time_entries.return_value = iter([{'id': 1}])
    api.base_url = 'http://redmine.example.com'
    api.__enter__.return_value = api
    api.__exit__.return_value = False
    RedmineAPI.return_value = api

    list(TimeEntries(RedminePlugin().name).collect({}, context))

    api.time_entries.assert_called_once_with('1')


@patch('redmine.streams.issue_journals.RedmineAPI')
def test_collect_issue_journals_skips_when_disabled(RedmineAPI):
    context = _collect_context()
    collected = list(IssueJournals(RedminePlugin().name).collect({}, context))

    assert collected == []
    RedmineAPI.assert_not_called()


@patch('redmine.streams.issue_journals.RedmineAPI')
def test_collect_issue_journals_fetches_each_issue_with_journals(RedmineAPI):
    context = _collect_context(collect_journals=True)
    api = MagicMock()
    api.issues.return_value = iter([{'id': 42}, {'id': 43}])
    api.issue.side_effect = [
        {'id': 42, 'journals': [{'id': 1, 'notes': 'a', 'details': []}]},
        {'id': 43, 'journals': []},
    ]
    api.base_url = 'http://redmine.example.com'
    api.__enter__.return_value = api
    api.__exit__.return_value = False
    RedmineAPI.return_value = api

    collected = list(IssueJournals(RedminePlugin().name).collect({}, context))

    assert len(collected) == 1
    journal, _ = collected[0]
    assert journal['id'] == 1
    assert journal['_issue_id'] == 42
    api.issues.assert_called_once_with(
        '1',
        created_on=None,
        updated_since=None,
    )
    assert api.issue.call_args_list == [
        ((42,), {'include_journals': True}),
        ((43,), {'include_journals': True}),
    ]
