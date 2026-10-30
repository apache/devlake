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

import pytest

from pydevlake.testing import ContextBuilder

from zephyr.main import ZephyrPlugin
from zephyr.models import TestCase, TestExecution
from zephyr.streams.test_cases import TestCases
from zephyr.streams.test_case_issue_links import TestCaseIssueLinks
from zephyr.streams.test_cycles import TestCycles
from zephyr.streams.test_execution_issue_links import TestExecutionIssueLinks
from zephyr.streams.test_executions import TestExecutions


def test_extract_maps_collected_test_execution_metadata():
    stream = TestExecutions(ZephyrPlugin().name)
    raw = {
        'id': 200,
        'key': 'ABC-E1',
        'projectKey': 'ABC',
        'testCaseId': 100,
        'testCycleId': 50,
        'testExecutionStatusId': 7,
        'testExecutionStatusName': 'Pass',
        'jiraProjectVersionId': 3,
        'jiraProjectVersionName': 'Release 2.0',
        'actualEndDate': '2024-01-15T10:00:00.000Z',
        'executedById': 'user-1',
        'assignedToId': 'user-2',
        'issueLinksJson': '[{"id": 1, "issueId": 500, "type": "RELATED"}]',
    }
    execution = stream.extract(raw)
    assert execution.test_case_id == 100
    assert execution.test_cycle_id == 50
    assert execution.jira_project_version_name == 'Release 2.0'
    assert execution.test_execution_status_name == 'Pass'
    assert execution.executed_by_id == 'user-1'
    assert execution.assigned_to_id == 'user-2'
    assert execution.issue_links_json is not None
    assert execution.actual_end_date is not None


def test_enrich_preserves_flat_execution_actor_ids():
    enriched = TestExecutions._enrich_test_execution(
        {
            'id': 200,
            'testCase': {'id': 100, 'key': 'ABC-T1'},
            'testCycle': {'id': 50, 'key': 'ABC-C1', 'name': 'Sprint 1'},
            'testExecutionStatus': {'id': 7},
            'jiraProjectVersion': {'id': 3, 'name': 'Release 2.0'},
            'executedById': 'user-flat-1',
            'assignedToId': 'user-flat-2',
            'links': {'issues': [{'id': 1, 'issueId': 500, 'type': 'RELATED'}]},
            'customFields': {'Env': 'QA'},
        },
        lookups=None,
        project_key='ABC',
    )
    assert enriched['executedById'] == 'user-flat-1'
    assert enriched['assignedToId'] == 'user-flat-2'
    assert enriched['testCaseId'] == 100
    assert enriched['testCycleId'] == 50
    assert enriched['issueLinksJson'] is not None


def test_enrich_falls_back_to_nested_execution_actor_refs():
    enriched = TestExecutions._enrich_test_execution(
        {
            'id': 200,
            'testCase': {'id': 100, 'key': 'ABC-T1'},
            'testCycle': {'id': 50, 'key': 'ABC-C1', 'name': 'Sprint 1'},
            'testExecutionStatus': {'id': 7},
            'executedBy': {'accountId': 'user-1'},
            'assignedTo': {'accountId': 'user-2'},
        },
        lookups=None,
        project_key='ABC',
    )
    assert enriched['executedById'] == 'user-1'
    assert enriched['assignedToId'] == 'user-2'


def test_issue_link_substreams_require_link_id():
    plugin = ZephyrPlugin().name
    case_parent = TestCase(
        id=100,
        key='ABC-T1',
        issue_links_json='[{"issueId": 500}, {"id": 9, "issueId": 10100, "type": "COVERAGE"}]',
    )
    execution_parent = TestExecution(
        id=200,
        key='ABC-E1',
        issue_links_json='[{"issueId": 500}, {"id": 1, "issueId": 500, "type": "RELATED"}]',
    )
    case_links = list(TestCaseIssueLinks(plugin).collect({}, None, case_parent))
    execution_links = list(TestExecutionIssueLinks(plugin).collect({}, None, execution_parent))
    assert [raw['id'] for raw, _ in case_links] == [9]
    assert [raw['id'] for raw, _ in execution_links] == [1]


def test_extract_maps_collected_test_cycle_metadata():
    stream = TestCycles(ZephyrPlugin().name)
    raw = {
        'id': 50,
        'key': 'ABC-C1',
        'name': 'Sprint 1 cycle',
        'description': 'Regression cycle',
        'projectKey': 'ABC',
        'folderId': 10,
        'folderName': 'Release folder',
        'statusId': 3,
        'statusName': 'Done',
        'jiraProjectVersionId': 7,
        'jiraProjectVersionName': 'Release 2.0',
        'plannedStartDate': '2024-01-01T00:00:00.000Z',
        'plannedEndDate': '2024-01-31T00:00:00.000Z',
    }
    cycle = stream.extract(raw)
    assert cycle.key == 'ABC-C1'
    assert cycle.folder_name == 'Release folder'
    assert cycle.status_name == 'Done'
    assert cycle.jira_project_version_name == 'Release 2.0'
    assert cycle.planned_start_date is not None
    assert cycle.planned_end_date is not None


def test_extract_maps_collected_test_case_metadata():
    stream = TestCases(ZephyrPlugin().name)
    raw = {
        'id': 100,
        'key': 'ABC-T1',
        'name': 'Login test',
        'projectKey': 'ABC',
        'folderId': 10,
        'folderName': 'Regression',
        'priorityId': 2,
        'priorityName': 'High',
        'statusId': 3,
        'statusName': 'Approved',
        'componentId': 4,
        'componentName': 'Checkout',
        'labelsJson': '["Smoke"]',
        'customFieldsJson': '{"Test Type": "Manual"}',
        'issueLinksJson': '[{"id": 9, "issueId": 10100, "type": "COVERAGE"}]',
    }
    test_case = stream.extract(raw)
    assert test_case.folder_name == 'Regression'
    assert test_case.priority_name == 'High'
    assert test_case.component_name == 'Checkout'
    assert test_case.status_name == 'Approved'
    assert test_case.labels_json is not None
    assert 'Smoke' in test_case.labels_json
    assert test_case.custom_fields_json is not None


def test_convert_uses_configured_test_type_custom_field():
    context = (
        ContextBuilder(ZephyrPlugin())
        .with_connection(token='token', endpoint='https://api.zephyrscale.smartbear.com/v2')
        .with_scope(id='ABC', name='ABC', project_key='ABC')
        .with_scope_config(test_type_custom_field='Test Type')
        .build()
    )
    stream = TestCases(ZephyrPlugin().name)
    test_case = stream.extract({
        'id': 100,
        'key': 'ABC-T1',
        'name': 'Login test',
        'projectKey': 'ABC',
        'customFieldsJson': '{"Test Type": "Manual"}',
    })
    qa_test_case = next(stream.convert(test_case, context))
    assert qa_test_case.type == 'Manual'


def test_convert_derives_start_time_from_execution_duration():
    context = (
        ContextBuilder(ZephyrPlugin())
        .with_connection(token='token', endpoint='https://api.zephyrscale.smartbear.com/v2')
        .with_scope(id='ABC', name='ABC', project_key='ABC')
        .with_scope_config()
        .build()
    )
    finish = datetime.datetime(2026, 7, 29, 16, 55, 24, tzinfo=datetime.timezone.utc)
    execution = TestExecution(
        id=42,
        test_case_id=100,
        actual_end_date=finish,
        execution_time=8000,
        test_execution_status_name='Pass',
    )
    domain_execution = next(TestExecutions(ZephyrPlugin().name).convert(execution, context))
    assert domain_execution.finish_time == finish
    assert domain_execution.create_time == finish
    assert domain_execution.start_time == finish - datetime.timedelta(milliseconds=8000)


@pytest.mark.parametrize(
    ('status_name', 'expected_status'),
    [
        ('Pass', 'SUCCESS'),
        ('Fail', 'FAILED'),
        ('Blocked', 'FAILED'),
        ('In Progress', 'IN_PROGRESS'),
        (None, 'PENDING'),
    ],
)
def test_convert_maps_execution_status_to_domain(status_name, expected_status):
    context = (
        ContextBuilder(ZephyrPlugin())
        .with_connection(token='token', endpoint='https://api.zephyrscale.smartbear.com/v2')
        .with_scope(id='ABC', name='ABC', project_key='ABC')
        .with_scope_config()
        .build()
    )
    execution = TestExecution(
        id=42,
        test_case_id=100,
        test_execution_status_name=status_name,
    )
    stream = TestExecutions(ZephyrPlugin().name)
    domain_execution = next(stream.convert(execution, context))
    assert domain_execution.status == expected_status
