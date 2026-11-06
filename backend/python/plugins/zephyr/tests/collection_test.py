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

from zephyr.main import ZephyrPlugin
from zephyr.streams.test_cases import TestCases
from zephyr.streams.test_cycles import TestCycles
from zephyr.streams.test_executions import TestExecutions


def _context(**kwargs):
    builder = (
        ContextBuilder(ZephyrPlugin())
        .with_connection(token='token', endpoint='https://api.zephyrscale.smartbear.com/v2')
        .with_scope(id='ABC', name='ABC', project_key='ABC')
        .with_scope_config()
    )
    if 'time_after' in kwargs:
        builder = builder.with_time_after(kwargs['time_after'])
    context = builder.build()
    if kwargs.get('incremental'):
        context.options['incremental'] = True
    return context


def _api_mock(iter_method: str, values: list):
    api = MagicMock()
    api.base_url = 'https://api.zephyrscale.smartbear.com/v2'
    api.project_lookups.return_value = MagicMock(
        folders={},
        priorities={},
        statuses={},
        components={},
        execution_statuses={},
        cycle_statuses={},
    )
    getattr(api, iter_method).return_value = iter(values)
    api.__enter__.return_value = api
    api.__exit__.return_value = False
    return api


@patch('zephyr.streams.test_cases.ZephyrAPI')
def test_test_cases_filters_by_project_time_after(ZephyrAPI):
    context = _context(time_after=datetime.datetime(2024, 6, 1, tzinfo=datetime.timezone.utc))
    api = _api_mock('iter_test_cases_nextgen', [
        {'id': 1, 'createdOn': '2024-01-01T00:00:00.000Z', 'name': 'old'},
        {'id': 2, 'createdOn': '2024-07-01T00:00:00.000Z', 'name': 'new'},
        {'id': 3, 'name': 'undated'},
    ])
    ZephyrAPI.return_value = api

    collected = list(TestCases(ZephyrPlugin().name).collect({}, context))

    assert [raw['id'] for raw, _ in collected] == [2, 3]
    assert collected[0][0]['x_request_input']['createdAfter'] == '2024-06-01T00:00:00+00:00'


@patch('zephyr.streams.test_cases.ZephyrAPI')
def test_test_cases_incremental_recollects_all(ZephyrAPI):
    context = _context(
        time_after=datetime.datetime(2024, 6, 1, tzinfo=datetime.timezone.utc),
        incremental=True,
    )
    api = _api_mock('iter_test_cases_nextgen', [
        {'id': 1, 'createdOn': '2024-01-01T00:00:00.000Z', 'name': 'old'},
        {'id': 2, 'createdOn': '2024-07-01T00:00:00.000Z', 'name': 'new'},
    ])
    ZephyrAPI.return_value = api

    collected = list(TestCases(ZephyrPlugin().name).collect({}, context))

    assert [raw['id'] for raw, _ in collected] == [1, 2]
    assert 'createdAfter' not in collected[0][0]['x_request_input']


@patch('zephyr.streams.test_cycles.ZephyrAPI')
def test_test_cycles_filters_by_project_time_after(ZephyrAPI):
    context = _context(time_after=datetime.datetime(2024, 6, 1, tzinfo=datetime.timezone.utc))
    api = _api_mock('iter_test_cycles_nextgen', [
        {'id': 1, 'plannedStartDate': '2024-01-01T00:00:00.000Z', 'name': 'old'},
        {'id': 2, 'plannedStartDate': '2024-07-01T00:00:00.000Z', 'name': 'new'},
    ])
    ZephyrAPI.return_value = api

    collected = list(TestCycles(ZephyrPlugin().name).collect({}, context))

    assert [raw['id'] for raw, _ in collected] == [2]


@patch('zephyr.streams.test_cycles.ZephyrAPI')
def test_test_cycles_incremental_recollects_all(ZephyrAPI):
    context = _context(
        time_after=datetime.datetime(2024, 6, 1, tzinfo=datetime.timezone.utc),
        incremental=True,
    )
    api = _api_mock('iter_test_cycles_nextgen', [
        {'id': 1, 'plannedStartDate': '2024-01-01T00:00:00.000Z', 'name': 'old'},
        {'id': 2, 'plannedStartDate': '2024-07-01T00:00:00.000Z', 'name': 'new'},
    ])
    ZephyrAPI.return_value = api

    collected = list(TestCycles(ZephyrPlugin().name).collect({}, context))

    assert [raw['id'] for raw, _ in collected] == [1, 2]


@patch('zephyr.streams.test_executions.ZephyrAPI')
def test_test_executions_uses_project_time_after(ZephyrAPI):
    context = _context(time_after=datetime.datetime(2024, 6, 1, tzinfo=datetime.timezone.utc))
    api = _api_mock('iter_test_executions_nextgen', [
        {'id': 1, 'actualEndDate': '2024-07-01T10:00:00.000Z'},
    ])
    ZephyrAPI.return_value = api

    _, state = next(TestExecutions(ZephyrPlugin().name).collect({}, context))

    api.iter_test_executions_nextgen.assert_called_once_with(
        'ABC',
        actual_end_date_after='2024-06-01T00:00:00Z',
    )
    assert state['actual_end_date_after'] == '2024-07-01T10:00:00.000Z'


@patch('zephyr.streams.test_executions.ZephyrAPI')
def test_test_executions_uses_bookmark_when_incremental(ZephyrAPI):
    context = _context(
        time_after=datetime.datetime(2024, 1, 1, tzinfo=datetime.timezone.utc),
        incremental=True,
    )
    api = _api_mock('iter_test_executions_nextgen', [
        {'id': 1, 'actualEndDate': '2024-07-02T10:00:00.000Z'},
    ])
    ZephyrAPI.return_value = api

    _, state = next(TestExecutions(ZephyrPlugin().name).collect(
        {'actual_end_date_after': '2024-07-01T00:00:00.000Z'},
        context,
    ))

    api.iter_test_executions_nextgen.assert_called_once_with(
        'ABC',
        actual_end_date_after='2024-07-01T00:00:00.000Z',
    )
    assert state['actual_end_date_after'] == '2024-07-02T10:00:00.000Z'


@patch('zephyr.streams.test_executions.ZephyrAPI')
def test_test_executions_without_time_after_omits_filter(ZephyrAPI):
    context = _context()
    api = _api_mock('iter_test_executions_nextgen', [])
    ZephyrAPI.return_value = api

    list(TestExecutions(ZephyrPlugin().name).collect({}, context))

    api.iter_test_executions_nextgen.assert_called_once_with(
        'ABC',
        actual_end_date_after=None,
    )
