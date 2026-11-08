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
import json
from unittest.mock import MagicMock

import pytest
from pydevlake.model import raw_data_params
from sqlmodel import Session

from glpi.collection import (
    prepare_on_demand_raw,
    referenced_group_ids,
    referenced_user_ids,
)
from glpi.main import GlpiPlugin
from glpi.models import Ticket
from glpi.streams import ticket_substreams
from glpi.streams.ticket_substreams import TicketLinks, TicketLinksCollector
from glpi.streams.tickets import Tickets
from tests.common import GLPI_API_BASE_URL, collect_context, glpi_api_mock


def test_collect_normalizes_bare_user_fields():
    with glpi_api_mock() as api:
        api.iter_tickets.return_value = iter([{
            'id': 1,
            'date_mod': '2024-01-01',
            'users_id_recipient': 101,
            'users_id_lastupdater': 102,
        }])
        api.get_item.side_effect = lambda itemtype, item_id: {
            'id': item_id, 'name': f'user{item_id}',
        }
        collected = [
            raw for raw, _ in Tickets(GlpiPlugin().name).collect({}, collect_context())
        ]

    assert collected[0]['users_id_recipient'] == 'user101 (101)'
    assert collected[0]['users_id_lastupdater'] == 'user102 (102)'


def test_collect_skips_zero_catalog_id():
    with glpi_api_mock() as api:
        api.iter_tickets.return_value = iter([{
            'id': 1,
            'date_mod': '2024-01-01',
            'locations_id': 0,
            'requesttypes_id': 2,
        }])
        api.get_item.side_effect = lambda itemtype, item_id: {'name': 'Email'}
        collected = [
            raw for raw, _ in Tickets(GlpiPlugin().name).collect({}, collect_context())
        ]

    assert collected[0]['locations_id'] == 0
    assert collected[0]['requesttypes_id'] == 'Email (2)'
    api.get_item.assert_called_once_with('RequestType', 2)


def test_collect_resolves_catalog_completename():
    with glpi_api_mock() as api:
        api.iter_tickets.return_value = iter([{
            'id': 1,
            'date_mod': '2024-01-01',
            'locations_id': 5,
        }])
        api.get_item.side_effect = lambda itemtype, item_id: {
            'completename': 'Root > Building A',
            'name': 'Building A',
        }
        collected = [
            raw for raw, _ in Tickets(GlpiPlugin().name).collect({}, collect_context())
        ]

    assert collected[0]['locations_id'] == 'Root > Building A (5)'
    api.get_item.assert_called_once_with('Location', 5)


def test_referenced_user_ids_from_raw_rows():
    plugin = GlpiPlugin()
    ctx = collect_context()

    with Session(ctx.engine) as session:
        _seed_raw(session, plugin, ctx, {
            'tickets': [{'id': 1, 'users_id_recipient': 'Requester (101)'}],
            'ticketusers': [{'id': 2, 'users_id': 'Tech (102)'}],
            'tickettasks': [{'id': 3, 'users_id_tech': 'Worker (103)'}],
        })
        assert referenced_user_ids(session, plugin, ctx) == {101, 102, 103}


def _seed_raw(session, plugin, ctx, rows_by_stream):
    params = raw_data_params(ctx.connection.id, ctx.scope.id)
    for stream_name, rows in rows_by_stream.items():
        raw_model = plugin.get_stream(stream_name).raw_model(session)
        for row in rows:
            session.add(raw_model(params=params, data=json.dumps(row)))
    session.commit()


def test_referenced_group_ids_from_raw_rows():
    plugin = GlpiPlugin()
    ctx = collect_context()

    with Session(ctx.engine) as session:
        _seed_raw(session, plugin, ctx, {
            'ticketgroups': [{'id': 1, 'groups_id': 'Support (5)'}],
        })
        assert referenced_group_ids(session, plugin, ctx) == {5}


def test_collect_uses_project_time_after_when_not_incremental():
    from pydevlake.testing import ContextBuilder
    from glpi.main import GlpiPlugin

    context = (
        ContextBuilder(GlpiPlugin())
        .with_connection(
            endpoint=GLPI_API_BASE_URL,
            token='test_app_token',
            secret_key='test_user_token',
            timezone='UTC',
        )
        .with_scope('1', name='Test Entity')
        .with_time_after(datetime.datetime(2024, 1, 1, tzinfo=datetime.timezone.utc))
        .build()
    )

    with glpi_api_mock() as api:
        api.iter_tickets.return_value = iter([{'id': 1, 'date_mod': '2024-06-02'}])
        list(Tickets(GlpiPlugin().name).collect({}, context))

    api.iter_tickets.assert_called_once_with(
        entity_id='1',
        updated_since=None,
        created_after='2024-01-01 00:00:00',
    )


@pytest.mark.parametrize('raw,ticket_ids,expected', [
    ({'tickets_id_1': 1, 'tickets_id_2': 2}, {1, 3}, True),
    ({'tickets_id_1': 1, 'tickets_id_2': 2}, {3, 4}, False),
    ({'tickets_id1': 5, 'tickets_id2': 6}, {6}, True),
])
def test_link_involves_tickets(raw, ticket_ids, expected):
    collector = TicketLinksCollector(TicketLinks(GlpiPlugin().name))
    context = collect_context(collect_ticket_links=True)

    session = MagicMock()
    session.query.return_value.where.return_value.all.return_value = [
        Ticket(id=ticket_id, connection_id=context.connection.id)
        for ticket_id in ticket_ids
    ]

    with glpi_api_mock(ticket_substreams) as api:
        api.iter_ticket_links.return_value = iter([raw])
        collected = list(collector.fetch({}, session, context))

    if expected:
        assert len(collected) == 1
        result_raw, _ = collected[0]
        for key, value in raw.items():
            assert result_raw[key] == value
        assert result_raw['x_request_url'] == f'{GLPI_API_BASE_URL}/apirest.php/Ticket_Ticket'
        assert result_raw['x_request_input'] == {}
    else:
        assert collected == []


def test_prepare_on_demand_raw_skips_empty_and_invalid_ids():
    assert prepare_on_demand_raw({}, 7, 'User', GLPI_API_BASE_URL) is None
    assert prepare_on_demand_raw({'name': 'jdoe'}, 0, 'User', GLPI_API_BASE_URL) is None


def test_prepare_on_demand_raw_sets_id_and_url():
    prepared = prepare_on_demand_raw({'name': 'jdoe'}, 7, 'User', GLPI_API_BASE_URL)
    assert prepared['id'] == 7
    assert prepared['x_request_url'] == f'{GLPI_API_BASE_URL}/apirest.php/User/7'


def test_ticket_links_collector_skips_when_disabled():
    collector = TicketLinksCollector(TicketLinks(GlpiPlugin().name))
    context = collect_context(collect_ticket_links=False)

    with glpi_api_mock(ticket_substreams) as api:
        collected = list(collector.fetch({}, MagicMock(), context))

    assert collected == []
    api.assert_not_called()


@pytest.mark.parametrize('stream_class,flag_name', [
    ('TicketFollowups', 'collect_followups'),
    ('TicketTasks', 'collect_tasks'),
    ('TicketLogs', 'collect_logs'),
    ('TicketSolutions', 'collect_solutions'),
    ('TicketUsers', 'collect_assignees'),
    ('TicketGroups', 'collect_assignees'),
])
def test_optional_substream_skips_when_disabled(stream_class, flag_name):
    stream = getattr(ticket_substreams, stream_class)(GlpiPlugin().name)
    context = collect_context(**{flag_name: False})
    mock_api = MagicMock()
    mock_api.base_url = GLPI_API_BASE_URL
    mock_api.ticket_subitems.return_value = iter([{'id': 2}])
    parent = Ticket(id=1, connection_id=context.connection.id)

    collected = list(stream.collect({}, context, parent, mock_api))

    assert collected == []
    mock_api.ticket_subitems.assert_not_called()
