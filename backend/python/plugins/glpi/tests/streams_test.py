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

import pydevlake.domain_layer.crossdomain as crossdomain
import pydevlake.domain_layer.ticket as ticket
from pydevlake.model import domain_id
from pydevlake.testing import assert_stream_convert

from glpi.main import GlpiPlugin
from glpi.models import (
    Followup,
    Group,
    Ticket,
    TicketGroup,
    TicketLink,
    TicketLog,
    TicketTask,
    TicketUser,
    User,
)
from glpi.streams.ticket_domain import TicketDomain
from tests.common import GLPI_API_BASE_URL, collect_context


@pytest.fixture
def context():
    return collect_context(scope_completename='Root > Test Entity')


def test_tickets_stream(context):
    ticket_model = Ticket(
        id=1,
        name='Network connectivity issue',
        content='<p>Cannot connect to the server.</p>',
        date=datetime.datetime(2024, 1, 15, 8, 0, 0),
        date_creation=datetime.datetime(2024, 1, 15, 10, 0, 0),
        date_mod=datetime.datetime(2024, 1, 16, 12, 0, 0),
        solvedate=datetime.datetime(2024, 1, 17, 9, 0, 0),
        status=5,
        type=1,
        urgency=3,
        impact=3,
        priority=4,
        entities_id='1',
        itilcategories_id=99,
        category_name='Network',
        users_id_recipient=101,
        requester_name='Requester User',
        assignee_id=101,
        assignee_name='Tech User',
        parent_ticket_id=10,
        solution_content='<p>Replaced cable.</p>',
        time_to_resolve=datetime.datetime(2024, 1, 20, 17, 0, 0),
        actiontime=3600,
        solve_delay_stat=7200,
        takeintoaccount_delay_stat=600,
        connection_id=context.connection.id,
    )

    issue = ticket.Issue(
        url=f'{GLPI_API_BASE_URL}/front/ticket.form.php?id=1',
        issue_key='1',
        title='Network connectivity issue',
        description='<p>Cannot connect to the server.</p>\n\n--- Solution ---\n<p>Replaced cable.</p>',
        type='INCIDENT',
        original_type='Incident',
        status='DONE',
        original_status='Solved',
        created_date='2024-01-15 08:00:00',
        updated_date='2024-01-16 12:00:00',
        resolution_date='2024-01-17 09:00:00',
        due_date='2024-01-20 17:00:00',
        creator_id=domain_id(User, context.connection.id, 101),
        creator_name='Requester User',
        assignee_id=domain_id(User, context.connection.id, 101),
        assignee_name='Tech User',
        parent_issue_id=domain_id(Ticket, context.connection.id, 10),
        is_subtask=True,
        priority='High',
        urgency='Medium',
        severity='Medium',
        component='Network',
        original_project='1',
        lead_time_minutes=120,
    )
    issue_id = domain_id(Ticket, context.connection.id, 1)
    domain_models = list(TicketDomain(GlpiPlugin().name).convert(ticket_model, context))
    assert domain_models == [
        issue,
        ticket.BoardIssue(board_id=context.scope.domain_id(), issue_id=None),
        ticket.IssueLabel(issue_id=issue_id, label_name='Network'),
    ]


def test_tickets_stream_extract_returns_tool_model(context):
    raw = {
        'id': 1,
        'name': 'Network connectivity issue',
        'entities_id': 'Root > Test Entity (1)',
        'itilcategories_id': 'Network (99)',
        'users_id_recipient': 'Requester User (101)',
    }
    stream = GlpiPlugin().get_stream('tickets')
    tool_model = stream.extract(raw, context)
    assert tool_model is not None
    assert tool_model.id == 1
    assert tool_model.category_name == 'Network'
    assert tool_model.requester_name == 'Requester User'


def test_tickets_stream_new_status(context):
    raw = {
        'id': 10,
        'name': 'Printer not working',
        'content': None,
        'date': '2024-02-01 09:00:00',
        'date_creation': None,
        'date_mod': '2024-02-01 09:30:00',
        'solvedate': None,
        'closedate': None,
        'status': 1,
        'type': 2,
        'urgency': 1,
        'impact': 1,
        'priority': 1,
        'entities_id': '2',
        'itilcategories_id': None,
        'users_id_recipient': None,
        'users_id_lastupdater': None,
        'requesttypes_id': None,
        'time_to_resolve': None,
        'actiontime': None,
        'solve_delay_stat': None,
        'takeintoaccount_delay_stat': None,
        'x_request_url': f'{GLPI_API_BASE_URL}/apirest.php/Ticket',
        'x_request_input': {'EntityId': '1'},
    }

    issue = ticket.Issue(
        url=f'{GLPI_API_BASE_URL}/front/ticket.form.php?id=10',
        issue_key='10',
        title='Printer not working',
        description=None,
        type='TASK',
        original_type='Request',
        status='TODO',
        original_status='New',
        created_date='2024-02-01 09:00:00',
        updated_date='2024-02-01 09:30:00',
        resolution_date=None,
        due_date=None,
        creator_id=None,
        creator_name=None,
        assignee_id=None,
        assignee_name=None,
        parent_issue_id=None,
        priority='Very Low',
        urgency='Very Low',
        severity='Very Low',
        component=None,
        original_project='2',
        time_spent_minutes=None,
        lead_time_minutes=None,
    )
    expected = [issue, ticket.BoardIssue(board_id=context.scope.domain_id(), issue_id=issue.id)]
    assert_stream_convert(GlpiPlugin, 'ticketdomain', raw, expected, context)


@pytest.mark.parametrize('stream_name,tool_model,assignee_model,assignee_id,assignee_name', [
    (
        'ticketusers',
        TicketUser(id=10, tickets_id=1, type=2, users_id=101, user_name='Tech User'),
        User, 101, 'Tech User',
    ),
    (
        'ticketgroups',
        TicketGroup(id=11, tickets_id=1, type=2, groups_id=5, group_name='Support Team'),
        Group, 5, 'Support Team',
    ),
])
def test_ticket_assignees_from_subitems(
    context, stream_name, tool_model, assignee_model, assignee_id, assignee_name,
):
    tool_model.connection_id = context.connection.id
    expected = ticket.IssueAssignee(
        issue_id=domain_id(Ticket, context.connection.id, 1),
        assignee_id=domain_id(assignee_model, context.connection.id, assignee_id),
        assignee_name=assignee_name,
    )
    assert _convert_subitem(stream_name, tool_model, context) == [expected]


def _convert_subitem(stream_name, tool_model, context):
    stream = GlpiPlugin().get_stream(stream_name)
    return list(stream.convert(tool_model, context))


def test_ticket_followups_from_subitems(context):
    tool_model = Followup(
        id=5,
        items_id=1,
        users_id=103,
        content='Investigated the issue, awaiting vendor response.',
        date=datetime.datetime(2024, 1, 16, 9, 0, 0),
        date_mod=datetime.datetime(2024, 1, 16, 9, 30, 0),
        is_private=0,
        connection_id=context.connection.id,
    )
    results = _convert_subitem('ticketfollowups', tool_model, context)
    assert len(results) == 1
    assert results[0].issue_id == domain_id(Ticket, context.connection.id, 1)
    assert results[0].body == 'Investigated the issue, awaiting vendor response.'
    assert results[0].account_id == domain_id(User, context.connection.id, 103)
    assert results[0].created_date == datetime.datetime(2024, 1, 16, 9, 0)
    assert results[0].updated_date == datetime.datetime(2024, 1, 16, 9, 30)


def test_ticket_worklogs_from_subitems(context):
    tool_model = TicketTask(
        id=7,
        tickets_id=1,
        users_id=101,
        content='Replaced the network cable.',
        begin=datetime.datetime(2024, 1, 17, 8, 0, 0),
        end=datetime.datetime(2024, 1, 17, 9, 0, 0),
        actiontime=3600,
        connection_id=context.connection.id,
    )
    results = _convert_subitem('tickettasks', tool_model, context)
    assert len(results) == 1
    assert results[0].issue_id == domain_id(Ticket, context.connection.id, 1)
    assert results[0].author_id == domain_id(User, context.connection.id, 101)
    assert results[0].comment == 'Replaced the network cable.'
    assert results[0].time_spent_minutes == 60
    assert results[0].started_date == datetime.datetime(2024, 1, 17, 8, 0)
    assert results[0].logged_date == datetime.datetime(2024, 1, 17, 8, 0)


def test_ticket_changelogs_from_subitems_smoke(context):
    tool_model = TicketLog(
        id=100,
        tickets_id=1,
        itemtype='Ticket',
        itemtype_link='',
        linked_action=0,
        user_name='admin (1)',
        date_mod=datetime.datetime(2024, 1, 16, 10, 0, 0),
        id_search_option=12,
        old_value='New',
        new_value='Processing (assigned)',
        connection_id=context.connection.id,
    )
    expected = ticket.IssueChangelogs(
        id=domain_id(TicketLog, context.connection.id, 100),
        issue_id=domain_id(Ticket, context.connection.id, 1),
        author_id=domain_id(User, context.connection.id, 1),
        author_name='admin',
        field_id='12',
        field_name='status',
        original_from_value='New',
        original_to_value='Processing (assigned)',
        from_value='TODO',
        to_value='IN_PROGRESS',
        created_date=datetime.datetime(2024, 1, 16, 10, 0),
    )
    results = _convert_subitem('ticketlogs', tool_model, context)
    assert len(results) == 1
    assert results[0] == expected


@pytest.mark.parametrize('tool_model', [
    TicketLog(
        id=103,
        tickets_id=1,
        itemtype='Ticket',
        itemtype_link='User',
        linked_action=15,
        user_name='admin (1)',
        date_mod=datetime.datetime(2024, 1, 16, 11, 30, 0),
        id_search_option=0,
        old_value='',
        new_value='Jane Smith (103)',
    ),
    TicketLog(
        id=102,
        tickets_id=1,
        itemtype='Ticket',
        itemtype_link='ITILFollowup',
        linked_action=17,
        user_name='admin (1)',
        date_mod=datetime.datetime(2024, 1, 16, 12, 0, 0),
        id_search_option=0,
        old_value='',
        new_value='Added follow-up',
    ),
])
def test_ticket_changelogs_skips_non_changelog_entries(context, tool_model):
    tool_model.connection_id = context.connection.id
    assert _convert_subitem('ticketlogs', tool_model, context) == []


def test_ticket_links_from_subitems(context):
    tool_model = TicketLink(
        id=3,
        tickets_id=1,
        tickets_id_1=1,
        tickets_id_2=99,
        link=1,
        connection_id=context.connection.id,
    )
    expected = ticket.IssueRelationship(
        source_issue_id=domain_id(Ticket, context.connection.id, 1),
        target_issue_id=domain_id(Ticket, context.connection.id, 99),
        original_type='linked',
    )
    assert _convert_subitem('ticketlinks', tool_model, context) == [expected]


def test_groups_stream(context):
    raw = {
        'id': 5,
        'name': 'Support',
        'completename': 'Root > Support',
        'x_request_url': f'{GLPI_API_BASE_URL}/apirest.php/Group',
    }
    plugin = GlpiPlugin()
    stream = plugin.get_stream('groups')
    tool_model = stream.extract(raw)
    tool_model.connection_id = context.connection.id
    account = list(stream.convert(tool_model, context))[0]
    assert account.full_name == 'Root > Support'
    assert account.user_name == 'Support'
    assert account.email == 'group-5@glpi.local'


def test_users_stream(context):
    raw = {
        'id': 7,
        'name': 'jdoe',
        'firstname': 'John',
        'realname': 'Doe',
        'email': 'john@example.com',
        'x_request_url': f'{GLPI_API_BASE_URL}/apirest.php/User',
    }

    expected = crossdomain.Account(
        email='john@example.com',
        full_name='John Doe',
        user_name='jdoe',
        avatar_url='',
        organization='',
        created_date='2024-01-01 00:00:00',
        status=1,
    )
    plugin = GlpiPlugin()
    stream = plugin.get_stream('users')
    tool_model = stream.extract(raw)
    tool_model.connection_id = context.connection.id
    results = list(stream.convert(tool_model, context))
    account = results[0]
    assert account.email == expected.email
    assert account.full_name == expected.full_name
    assert account.user_name == expected.user_name
    assert account.status == expected.status


def test_users_stream_extracts_id_from_request_url(context):
    raw = {
        'x_request_url': f'{GLPI_API_BASE_URL}/apirest.php/User/7',
    }
    plugin = GlpiPlugin()
    stream = plugin.get_stream('users')
    tool_model = stream.extract(raw)
    assert tool_model.id == 7


def test_ticket_worklogs_skip_entries_without_time(context):
    tool_model = TicketTask(
        id=8,
        tickets_id=15,
        users_id=None,
        content=None,
        begin=None,
        end=None,
        actiontime=None,
        connection_id=context.connection.id,
    )
    assert _convert_subitem('tickettasks', tool_model, context) == []


def test_tickets_stream_converts_timezone(context):
    context.connection.timezone = 'America/Sao_Paulo'
    raw = {
        'id': 99,
        'name': 'Timezone test',
        'content': None,
        'date': '2026-06-02 00:00:00',
        'date_creation': None,
        'date_mod': '2026-06-02 01:00:00',
        'solvedate': None,
        'closedate': None,
        'status': 1,
        'type': 1,
        'urgency': 3,
        'impact': 3,
        'priority': 3,
        'entities_id': '1',
        'itilcategories_id': None,
        'users_id_recipient': None,
        'users_id_lastupdater': None,
        'requesttypes_id': None,
        'time_to_resolve': None,
        'actiontime': None,
        'solve_delay_stat': None,
        'takeintoaccount_delay_stat': None,
        'x_request_url': f'{GLPI_API_BASE_URL}/apirest.php/Ticket',
        'x_request_input': {'EntityId': '1'},
    }

    issue = ticket.Issue(
        url=f'{GLPI_API_BASE_URL}/front/ticket.form.php?id=99',
        issue_key='99',
        title='Timezone test',
        description=None,
        type='INCIDENT',
        original_type='Incident',
        status='TODO',
        original_status='New',
        created_date='2026-06-02 03:00:00',
        updated_date='2026-06-02 04:00:00',
        resolution_date=None,
        due_date=None,
        creator_id=None,
        creator_name=None,
        assignee_id=None,
        assignee_name=None,
        parent_issue_id=None,
        priority='Medium',
        urgency='Medium',
        severity='Medium',
        component=None,
        original_project='1',
        time_spent_minutes=None,
        lead_time_minutes=None,
    )
    expected = [issue, ticket.BoardIssue(board_id=context.scope.domain_id(), issue_id=issue.id)]
    assert_stream_convert(GlpiPlugin, 'ticketdomain', raw, expected, context)
