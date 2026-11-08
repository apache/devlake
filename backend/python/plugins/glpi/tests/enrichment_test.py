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

import json
from typing import Optional

import pytest
import pydevlake.domain_layer.ticket as ticket
from pydevlake.model import domain_id
from sqlmodel import Session

from glpi.main import GlpiPlugin
from glpi.streams.ticket_domain import TicketDomain
from glpi.streams.tickets import Tickets
from glpi.streams.ticket_substreams import TicketLogs
from glpi.models import (
    Group,
    ITILSolution,
    Ticket,
    TicketLink,
    TicketLog,
    TicketUser,
    User,
)
from tests.common import collect_context, ensure_stream_raw_table


def test_resolve_glpi_user_ref_from_lookup():
    context = collect_context()
    stream = Tickets(GlpiPlugin().name)

    User.__table__.create(context.engine, checkfirst=True)
    with Session(context.engine) as session:
        session.add(User(id=2, name='glpi', connection_id=context.connection.id))
        session.commit()
        ensure_stream_raw_table(context.engine, stream)
        list(stream.extractor.fetch({}, session, context))

    ticket_model = stream.extract({'id': 1, 'users_id_recipient': 'glpi'}, context)
    assert ticket_model.requester_name == 'glpi'
    assert ticket_model.users_id_recipient == 2


def _build_raw_ticket(props: dict) -> dict:
    return {
        'id': 1,
        'name': 'Issue',
        'itilcategories_id': 'Network (99)',
        'users_id_recipient': 'Requester User (101)',
    } | props


def test_apply_ticket_enrichment_with_bare_ids():
    context = collect_context(
        '2',
        scope_name='Systems',
        scope_completename='Test > Systems',
    )
    raw = _build_raw_ticket({'entities_id': 2})
    stream = Tickets(GlpiPlugin().name)
    ticket_model = stream.extract(raw, context)

    assert ticket_model.entities_id == 'Test > Systems'
    assert ticket_model.category_name == 'Network'
    assert ticket_model.requester_name == 'Requester User'


def test_apply_ticket_enrichment_leaves_raw_unchanged():
    custom_fields_dict = {'custom_plugin_field': 'value'}
    context = collect_context()
    raw = _build_raw_ticket(custom_fields_dict)
    raw_snapshot = dict(raw)

    stream = Tickets(GlpiPlugin().name)
    ticket_model = stream.extract(raw, context)

    assert raw == raw_snapshot
    assert ticket_model.category_name == 'Network'
    assert ticket_model.itilcategories_id == 99
    assert ticket_model.requester_name == 'Requester User'
    assert ticket_model.users_id_recipient == 101
    assert ticket_model.plugin_fields_json == json.dumps(custom_fields_dict)


def test_enrich_ticket_from_tool_records():
    context = collect_context()
    connection_id = context.connection.id
    stream = TicketDomain(GlpiPlugin().name)

    for model in (TicketUser, TicketLink, ITILSolution, *stream.domain_models):
        model.__table__.create(context.engine, checkfirst=True)

    with Session(context.engine) as session:
        session.add(TicketUser(
            id=1,
            connection_id=connection_id,
            tickets_id=1,
            type=2,
            users_id=101,
            user_name='Tech User',
        ))
        session.add(ITILSolution(
            id=1,
            connection_id=connection_id,
            tickets_id=1,
            content='<p>Fixed.</p>',
        ))
        session.commit()

        ticket_model = Ticket(id=1, name='Issue', connection_id=connection_id)
        stream.convertor.process(ticket_model, session, context)

    assert ticket_model.assignee_id == 101
    assert ticket_model.assignee_name == 'Tech User'
    assert ticket_model.solution_content == '<p>Fixed.</p>'



def _parent_ticket_ids_from_link(
    context,
    *,
    link_type: int,
    tickets_id_1: int,
    tickets_id_2: int,
    ticket_ids: list[int],
) -> dict[int, Optional[int]]:
    connection_id = context.connection.id
    stream = TicketDomain(GlpiPlugin().name)
    for model in (TicketUser, TicketLink, ITILSolution, *stream.domain_models):
        model.__table__.create(context.engine, checkfirst=True)

    parent_ids: dict[int, Optional[int]] = {}
    with Session(context.engine) as session:
        session.add(TicketLink(
            id=1,
            connection_id=connection_id,
            link=link_type,
            tickets_id_1=tickets_id_1,
            tickets_id_2=tickets_id_2,
        ))
        session.commit()
        for ticket_id in ticket_ids:
            ticket_model = Ticket(id=ticket_id, name='Issue', connection_id=connection_id)
            stream.convertor.process(ticket_model, session, context)
            parent_ids[ticket_id] = ticket_model.parent_ticket_id
    return parent_ids


@pytest.mark.parametrize('link_type,tickets_id_1,tickets_id_2,ticket_id,expected', [
    (3, 1, 10, 1, 10),
    (3, 1, 10, 10, None),
    (4, 10, 1, 1, 10),
    (4, 10, 1, 10, None),
])
def test_parent_ticket_id_from_links(link_type, tickets_id_1, tickets_id_2, ticket_id, expected):
    context = collect_context()
    parent_ids = _parent_ticket_ids_from_link(
        context,
        link_type=link_type,
        tickets_id_1=tickets_id_1,
        tickets_id_2=tickets_id_2,
        ticket_ids=[ticket_id],
    )
    assert parent_ids[ticket_id] == expected


@pytest.mark.parametrize('old_value,new_value,expected_from,expected_to', [
    ('', 'New', None, 'TODO'),
    ('', 'Solved', None, 'DONE'),
    ('', 'Processing (assigned)', None, 'IN_PROGRESS'),
    ('', '2', None, 'IN_PROGRESS'),
    ('', '5', None, 'DONE'),
    ('2', '3', 'IN_PROGRESS', 'IN_PROGRESS'),
])
def test_ticket_log_status_changelog(old_value, new_value, expected_from, expected_to):
    context = collect_context()
    log = TicketLog(
        id=1,
        tickets_id=1,
        linked_action=0,
        id_search_option=12,
        old_value=old_value,
        new_value=new_value,
    )
    changelogs = list(TicketLogs(GlpiPlugin().name).convert(log, context))
    assert len(changelogs) == 1
    changelog = changelogs[0]
    assert changelog.field_name == 'status'
    assert changelog.from_value == expected_from
    assert changelog.to_value == expected_to
    assert changelog.original_from_value == (old_value or None)
    assert changelog.original_to_value == new_value


def test_ticket_log_assignee_user_relation():
    context = collect_context()
    log = TicketLog(
        id=6,
        tickets_id=1,
        linked_action=15,
        itemtype_link='User',
        id_search_option=5,
        old_value='',
        new_value='glpi (2)',
    )
    changelogs = list(TicketLogs(GlpiPlugin().name).convert(log, context))
    assert len(changelogs) == 1
    changelog = changelogs[0]
    assert changelog.field_name == 'assignee'
    assert changelog.to_value == domain_id(User, context.connection.id, 2)


def test_ticket_log_assignee_group_relation():
    context = collect_context()
    log = TicketLog(
        id=6,
        tickets_id=1,
        linked_action=15,
        itemtype_link='Group',
        old_value='',
        new_value='Support Team (5)',
    )
    changelogs = list(TicketLogs(GlpiPlugin().name).convert(log, context))
    assert len(changelogs) == 1
    changelog = changelogs[0]
    assert changelog.field_name == 'assignee'
    assert changelog.to_value == domain_id(Group, context.connection.id, 5)


def test_ticket_log_assignee_field_change():
    context = collect_context()
    log = TicketLog(
        id=7,
        tickets_id=1,
        linked_action=0,
        id_search_option=5,
        old_value='',
        new_value='Jane Smith (103)',
    )
    changelogs = list(TicketLogs(GlpiPlugin().name).convert(log, context))
    assert len(changelogs) == 1
    changelog = changelogs[0]
    assert changelog.field_name == 'assignee'
    assert changelog.from_value is None
    assert changelog.to_value == domain_id(User, context.connection.id, 103)
    assert changelog.original_to_value == '103'


def test_ticket_log_requester_field_change():
    context = collect_context()
    log = TicketLog(
        id=104,
        tickets_id=1,
        linked_action=0,
        id_search_option=4,
        old_value='',
        new_value='Jane Smith (103)',
    )
    changelogs = list(TicketLogs(GlpiPlugin().name).convert(log, context))
    assert len(changelogs) == 1
    changelog = changelogs[0]
    assert changelog.field_name == 'requester'
    assert changelog.from_value is None
    assert changelog.to_value == domain_id(User, context.connection.id, 103)
    assert changelog.original_to_value == '103'


def test_ticket_log_priority_change():
    context = collect_context()
    log = TicketLog(
        id=4,
        tickets_id=1,
        linked_action=0,
        id_search_option=3,
        old_value='3',
        new_value='High',
    )
    changelogs = list(TicketLogs(GlpiPlugin().name).convert(log, context))
    assert len(changelogs) == 1
    changelog = changelogs[0]
    assert changelog.field_name == 'priority'
    assert changelog.from_value == 'Medium'
    assert changelog.to_value == 'High'


def test_ticket_log_type_change():
    context = collect_context()
    log = TicketLog(
        id=5,
        tickets_id=1,
        linked_action=0,
        id_search_option=14,
        old_value='Incident',
        new_value='Request',
    )
    changelogs = list(TicketLogs(GlpiPlugin().name).convert(log, context))
    assert len(changelogs) == 1
    changelog = changelogs[0]
    assert changelog.field_name == 'type'
    assert changelog.from_value == 'INCIDENT'
    assert changelog.to_value == 'TASK'
