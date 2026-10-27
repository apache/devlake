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

import pydevlake.domain_layer.ticket as ticket
from pydevlake.model import domain_id
from pydevlake.testing import assert_stream_convert, ContextBuilder

from redmine.main import RedminePlugin
from redmine.models import Issue, Journal, StatusMapping, TypeMapping


def _issue_raw(issue_id: int, tracker_name: str, status_name: str, subject: str = 'Test issue') -> dict:
    return {
        'id': issue_id,
        'project': {'id': 1, 'name': 'Test Project'},
        'tracker': {'id': 1, 'name': tracker_name},
        'status': {'id': 1, 'name': status_name},
        'priority': {'id': 3, 'name': 'Normal'},
        'author': {'id': 10, 'name': 'John Smith'},
        'subject': subject,
        'created_on': '2024-02-01T10:00:00Z',
        'updated_on': '2024-02-01T10:00:00Z',
        'x_request_url': 'http://redmine.example.com/issues.json',
        'x_request_input': {'project_id': '1'},
    }


def _expected_issue(context, issue_id, tracker_name, status_name, expected_type, expected_status, subject='Test issue'):
    i = ticket.Issue(
        url=f'http://redmine.example.com/issues/{issue_id}',
        issue_key=str(issue_id),
        title=subject,
        description=None,
        type=expected_type,
        original_type=tracker_name,
        status=expected_status,
        original_status=status_name,
        created_date='2024-02-01T10:00:00Z',
        updated_date='2024-02-01T10:00:00Z',
        creator_id='10',
        creator_name='John Smith',
        priority='Normal',
        original_project='Test Project',
    )
    return [i, ticket.BoardIssue(board_id=context.scope.domain_id(), issue_id=i.id)]


@pytest.fixture
def context():
    return (
        ContextBuilder(RedminePlugin())
        .with_connection(endpoint='http://redmine.example.com', token='test_api_key')
        .with_scope('1', name='Test Project')
        .with_scope_config()
        .build()
    )


@pytest.fixture
def context_with_journals():
    return (
        ContextBuilder(RedminePlugin())
        .with_connection(endpoint='http://redmine.example.com', token='test_api_key')
        .with_scope('1', name='Test Project')
        .with_scope_config(collect_journals=True)
        .build()
    )


def test_issues_stream(context):
    raw = {
        'id': 42,
        'project': {'id': 1, 'name': 'Test Project'},
        'tracker': {'id': 1, 'name': 'Bug'},
        'status': {'id': 5, 'name': 'Resolved'},
        'priority': {'id': 4, 'name': 'High'},
        'author': {'id': 10, 'name': 'John Smith'},
        'assigned_to': {'id': 11, 'name': 'Jane Doe'},
        'parent': {'id': 40, 'name': 'Parent issue'},
        'category': {'id': 9, 'name': 'Authentication'},
        'fixed_version': {'id': 3, 'name': '1.0.0'},
        'subject': 'Cannot login to the app',
        'description': 'Login fails with 500 error.',
        'start_date': '2024-01-10',
        'due_date': '2024-01-20',
        'done_ratio': 100,
        'estimated_hours': 4.0,
        'spent_hours': 2.5,
        'total_spent_hours': 3.0,
        'custom_fields': [
            {'id': 1, 'name': 'Resolution', 'value': 'Fixed'},
            {'id': 2, 'name': 'Tags', 'value': ['backend', 'security']},
        ],
        'relations': [
            {
                'id': 1819,
                'issue_id': 42,
                'issue_to_id': 43,
                'relation_type': 'blocks',
            }
        ],
        'created_on': '2024-01-10T09:00:00Z',
        'updated_on': '2024-01-15T14:00:00Z',
        'closed_on': '2024-01-15T14:00:00Z',
        'x_request_url': 'http://redmine.example.com/issues.json',
        'x_request_input': {'project_id': '1'},
    }

    issue_id = domain_id(Issue, context.connection.id, 42)
    i = ticket.Issue(
        url='http://redmine.example.com/issues/42',
        issue_key='42',
        title='Cannot login to the app',
        description='Login fails with 500 error.',
        type='INCIDENT',
        original_type='Bug',
        status='DONE',
        original_status='Resolved',
        created_date='2024-01-10T09:00:00Z',
        updated_date='2024-01-15T14:00:00Z',
        resolution_date='2024-01-15T14:00:00Z',
        due_date=datetime.datetime(2024, 1, 20),
        creator_id='10',
        creator_name='John Smith',
        assignee_id='11',
        assignee_name='Jane Doe',
        parent_issue_id=domain_id(Issue, context.connection.id, 40),
        is_subtask=True,
        priority='High',
        component='Authentication',
        lead_time_minutes=7500,
        original_project='Test Project',
        original_estimate_minutes=240,
        time_spent_minutes=150,
        time_remaining_minutes=90,
        fix_versions='1.0.0',
    )
    expected = [
        i,
        ticket.BoardIssue(board_id=context.scope.domain_id(), issue_id=i.id),
        ticket.IssueAssignee(issue_id=i.id, assignee_id='11', assignee_name='Jane Doe'),
        ticket.IssueLabel(issue_id=issue_id, label_name='Authentication'),
        ticket.IssueRelationship(
            source_issue_id=domain_id(Issue, context.connection.id, 42),
            target_issue_id=domain_id(Issue, context.connection.id, 43),
            original_type='blocks',
        ),
    ]
    assert_stream_convert(RedminePlugin, 'issues', raw, expected, context)


def test_issues_stream_due_date_as_string(context):
    raw = {
        'id': 55,
        'project': {'id': 1, 'name': 'Test Project'},
        'tracker': {'id': 2, 'name': 'Feature'},
        'status': {'id': 1, 'name': 'New'},
        'priority': {'id': 3, 'name': 'Normal'},
        'author': {'id': 10, 'name': 'John Smith'},
        'subject': 'Task with due date',
        'due_date': '2024-06-15',
        'created_on': '2024-02-01T10:00:00Z',
        'updated_on': '2024-02-01T10:00:00Z',
        'x_request_url': 'http://redmine.example.com/issues.json',
        'x_request_input': {'project_id': '1'},
    }

    stream = RedminePlugin().get_stream('issues')
    tool_model = stream.extract(raw)
    tool_model.connection_id = context.connection.id
    tool_model.due_date = '2024-06-15'

    domain_models = list(stream.convert(tool_model, context))
    assert domain_models[0].due_date == datetime.datetime(2024, 6, 15)


def test_issues_stream_no_assignee(context):
    raw = {
        'id': 10,
        'project': {'id': 1, 'name': 'Test Project'},
        'tracker': {'id': 2, 'name': 'Feature'},
        'status': {'id': 1, 'name': 'New'},
        'priority': {'id': 3, 'name': 'Normal'},
        'author': {'id': 10, 'name': 'John Smith'},
        'subject': 'Add dark mode',
        'description': None,
        'start_date': None,
        'due_date': None,
        'done_ratio': 0,
        'estimated_hours': None,
        'created_on': '2024-02-01T10:00:00Z',
        'updated_on': '2024-02-01T10:00:00Z',
        'closed_on': None,
        'x_request_url': 'http://redmine.example.com/issues.json',
        'x_request_input': {'project_id': '1'},
    }

    i = ticket.Issue(
        url='http://redmine.example.com/issues/10',
        issue_key='10',
        title='Add dark mode',
        description=None,
        type='REQUIREMENT',
        original_type='Feature',
        status='TODO',
        original_status='New',
        created_date='2024-02-01T10:00:00Z',
        updated_date='2024-02-01T10:00:00Z',
        resolution_date=None,
        creator_id='10',
        creator_name='John Smith',
        priority='Normal',
        original_project='Test Project',
    )
    expected = [i, ticket.BoardIssue(board_id=context.scope.domain_id(), issue_id=i.id)]
    assert_stream_convert(RedminePlugin, 'issues', raw, expected, context)


@pytest.mark.parametrize('tracker_name,expected_type', [
    ('Bug', 'INCIDENT'),
    ('Feature', 'REQUIREMENT'),
    ('Custom', 'TASK'),
])
def test_issues_type_mapping_defaults(context, tracker_name, expected_type):
    raw = _issue_raw(1, tracker_name, 'New')
    expected = _expected_issue(context, 1, tracker_name, 'New', expected_type, 'TODO')
    assert_stream_convert(RedminePlugin, 'issues', raw, expected, context)


@pytest.mark.parametrize('status_name,expected_status', [
    ('New', 'TODO'),
    ('Closed', 'DONE'),
    ('On Hold', 'OTHER'),
])
def test_issues_status_mapping_defaults(context, status_name, expected_status):
    raw = _issue_raw(2, 'Task', status_name)
    expected = _expected_issue(context, 2, 'Task', status_name, 'TASK', expected_status)
    assert_stream_convert(RedminePlugin, 'issues', raw, expected, context)


def test_issues_type_mapping_from_scope_config():
    context = (
        ContextBuilder(RedminePlugin())
        .with_connection(endpoint='http://redmine.example.com', token='test_api_key')
        .with_scope('1', name='Test Project')
        .with_scope_config(type_mappings={'Defect': TypeMapping(standard_type='BUG')})
        .build()
    )
    raw = _issue_raw(3, 'Defect', 'New')
    expected = _expected_issue(context, 3, 'Defect', 'New', 'BUG', 'TODO')
    assert_stream_convert(RedminePlugin, 'issues', raw, expected, context)


def test_issues_status_mapping_from_scope_config():
    context = (
        ContextBuilder(RedminePlugin())
        .with_connection(endpoint='http://redmine.example.com', token='test_api_key')
        .with_scope('1', name='Test Project')
        .with_scope_config(status_mappings={'En cours': StatusMapping(standard_status='IN_PROGRESS')})
        .build()
    )
    raw = _issue_raw(4, 'Task', 'En cours')
    expected = _expected_issue(context, 4, 'Task', 'En cours', 'TASK', 'IN_PROGRESS')
    assert_stream_convert(RedminePlugin, 'issues', raw, expected, context)


def test_issues_stream_custom_mappings():
    context = (
        ContextBuilder(RedminePlugin())
        .with_connection(endpoint='http://redmine.example.com', token='test_api_key')
        .with_scope('1', name='Test Project')
        .with_scope_config(
            type_mappings={'Defect': {'standardType': 'BUG'}},
            status_mappings={'Ouvert': {'standardStatus': 'TODO'}},
        )
        .build()
    )
    raw = _issue_raw(99, 'Defect', 'Ouvert', subject='Broken button')
    expected = _expected_issue(context, 99, 'Defect', 'Ouvert', 'BUG', 'TODO', subject='Broken button')
    assert_stream_convert(RedminePlugin, 'issues', raw, expected, context)


def test_issue_journals_stream(context_with_journals):
    raw = {
        'id': 5,
        '_issue_id': 42,
        'user': {'id': 11, 'name': 'Jane Doe'},
        'notes': 'Investigated - network issue found.',
        'created_on': '2024-01-12T11:00:00Z',
        'private_notes': False,
        'details': [],
        'x_request_url': 'http://redmine.example.com/issues/42.json',
    }

    expected = ticket.IssueComment(
        issue_id=domain_id(Issue, context_with_journals.connection.id, 42),
        body='Investigated - network issue found.',
        account_id='11',
        created_date='2024-01-12T11:00:00Z',
    )
    assert_stream_convert(RedminePlugin, 'issuejournals', raw, expected, context_with_journals)


def test_issue_journals_changelog_stream(context_with_journals):
    raw = {
        'id': 6,
        '_issue_id': 42,
        'user': {'id': 11, 'name': 'Jane Doe'},
        'notes': '',
        'created_on': '2024-01-13T11:00:00Z',
        'private_notes': False,
        'details': [
            {
                'property': 'attr',
                'name': 'assigned_to_id',
                'old_value': '10',
                'new_value': '11',
            },
            {
                'property': 'attr',
                'name': 'status_id',
                'old_value': '1',
                'new_value': '2',
            },
        ],
        'x_request_url': 'http://redmine.example.com/issues/42.json',
    }

    issue_id = domain_id(Issue, context_with_journals.connection.id, 42)
    expected = [
        ticket.IssueChangelogs(
            id=domain_id(Journal, context_with_journals.connection.id, 6, 0),
            issue_id=issue_id,
            author_id='11',
            author_name='Jane Doe',
            field_id='assigned_to_id',
            field_name='assignee',
            original_from_value='10',
            original_to_value='11',
            from_value='10',
            to_value='11',
            created_date='2024-01-13T11:00:00Z',
        ),
        ticket.IssueChangelogs(
            id=domain_id(Journal, context_with_journals.connection.id, 6, 1),
            issue_id=issue_id,
            author_id='11',
            author_name='Jane Doe',
            field_id='status_id',
            field_name='status',
            original_from_value='1',
            original_to_value='2',
            from_value='1',
            to_value='2',
            created_date='2024-01-13T11:00:00Z',
        ),
    ]
    assert_stream_convert(RedminePlugin, 'issuejournals', raw, expected, context_with_journals)


def test_time_entries_stream(context):
    raw = {
        'id': 7,
        'project': {'id': 1, 'name': 'Test Project'},
        'issue': {'id': 42},
        'user': {'id': 10, 'name': 'John Smith'},
        'hours': 2.5,
        'comments': 'Fixed the login bug.',
        'spent_on': '2024-01-14',
        'created_on': '2024-01-14T17:00:00Z',
        'updated_on': '2024-01-14T17:00:00Z',
        'x_request_url': 'http://redmine.example.com/time_entries.json',
    }

    expected = ticket.IssueWorklog(
        issue_id=domain_id(Issue, context.connection.id, 42),
        author_id='10',
        comment='Fixed the login bug.',
        time_spent_minutes=150,
        started_date='2024-01-14',
        logged_date='2024-01-14T17:00:00Z',
    )
    assert_stream_convert(RedminePlugin, 'timeentries', raw, expected, context)


def test_time_entries_stream_no_issue(context):
    raw = {
        'id': 8,
        'project': {'id': 1, 'name': 'Test Project'},
        'user': {'id': 10, 'name': 'John Smith'},
        'hours': 1.0,
        'comments': 'General project work',
        'spent_on': '2024-01-15',
        'created_on': '2024-01-15T18:00:00Z',
        'updated_on': '2024-01-15T18:00:00Z',
        'x_request_url': 'http://redmine.example.com/time_entries.json',
    }

    assert_stream_convert(RedminePlugin, 'timeentries', raw, [], context)
