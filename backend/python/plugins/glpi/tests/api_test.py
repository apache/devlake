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

from pydevlake.api import APIException

from glpi.api import GlpiAPI, _TICKET_SORT_DATE_MOD, _TICKET_SORT_OPENING_DATE


def _mock_response(body, content_range='0-99/100'):
    response = MagicMock()
    response.json = body
    response.headers = {'Content-Range': content_range}
    return response


class PatchedGlpiAPI:
    _AUTO_MOCK_METHODS = frozenset({'get', '_api_get'})

    def __init__(self):
        object.__setattr__(self, '_api', GlpiAPI(MagicMock()))
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
def _patched_api() -> Iterator[PatchedGlpiAPI]:
    with PatchedGlpiAPI() as api:
        yield api


def test_iter_ticket_list_caps_range_on_first_request():
    with _patched_api() as api:
        api.get.return_value = _mock_response([{'id': i} for i in range(1, 6)], '0-4/5')
        collected = list(api._iter_ticket_list(limit=5))

    assert [ticket['id'] for ticket in collected] == [1, 2, 3, 4, 5]
    api.get.assert_called_once()
    assert api.get.call_args.kwargs['range'] == '0-4'
    assert api.get.call_args.kwargs['expand_dropdowns'] == 'true'


def test_iter_ticket_list_stops_after_limit_across_pages():
    page_one = [{'id': i} for i in range(1, 101)]
    page_two = [{'id': i} for i in range(101, 106)]

    with _patched_api() as api:
        api.get.side_effect = [
            _mock_response(page_one, '0-99/105'),
            _mock_response(page_two, '100-104/105'),
        ]
        collected = list(api._iter_ticket_list(limit=105))

    assert len(collected) == 105
    assert api.get.call_count == 2
    assert api.get.call_args_list[0].kwargs['range'] == '0-99'
    assert api.get.call_args_list[1].kwargs['range'] == '100-104'


def test_iter_tickets_stops_at_updated_since():
    tickets = [
        {'id': 1, 'date_mod': '2024-02-01 10:00:00'},
        {'id': 2, 'date_mod': '2024-01-01 10:00:00'},
        {'id': 3, 'date_mod': '2023-12-01 10:00:00'},
    ]

    with _patched_api() as api:
        api.get.return_value = _mock_response(tickets, '0-2/3')
        collected = list(api.iter_tickets(updated_since='2024-01-15 00:00:00'))

    assert [ticket['id'] for ticket in collected] == [1]
    assert api.get.call_args.kwargs['sort'] == _TICKET_SORT_DATE_MOD
    assert api.get.call_args.kwargs['order'] == 'DESC'


def test_iter_tickets_created_after_stops_at_opening_date():
    tickets = [
        {'id': 1, 'date': '2024-02-01 10:00:00'},
        {'id': 2, 'date': '2024-01-01 10:00:00'},
    ]

    with _patched_api() as api:
        api.get.return_value = _mock_response(tickets, '0-1/2')
        collected = list(api.iter_tickets(created_after='2024-01-15 00:00:00'))

    assert [ticket['id'] for ticket in collected] == [1]
    assert api.get.call_args.kwargs['sort'] == _TICKET_SORT_OPENING_DATE


def test_get_item_skips_zero_ids():
    users = [{'id': 1, 'name': 'user1'}]

    with _patched_api() as api:
        api.get.return_value = _mock_response(users)
        assert api.get_item('Location', 0) == {}
        assert api.get_item('User', 1) == users[0]

    api.get.assert_called_once_with(
        'apirest.php', 'User', '1',
        expand_dropdowns='false', get_hateoas='false',
    )


def test_get_item_returns_empty_for_missing_item():
    users = [{'id': 1, 'name': 'user1'}]

    def _get(*args, **kwargs):
        if args[1] == 'Location':
            response = MagicMock()
            response.status = 404
            response.body = b'["ERROR_ITEM_NOT_FOUND","Item not found"]'
            response.json = response.body
            response.headers = {}
            response.request = MagicMock()
            raise APIException(response)
        return _mock_response(users)

    with _patched_api() as api:
        api.get.side_effect = _get
        assert api.get_item('User', 1) == users[0]
        assert api.get_item('Location', 999) == {}


def test_iter_tickets_incremental_uses_paginated_list():
    tickets = [
        {'id': 1, 'date_mod': '2024-02-01 10:00:00'},
        {'id': 2, 'date_mod': '2024-02-02 10:00:00'},
    ]

    with _patched_api() as api:
        api.get.return_value = _mock_response(tickets, '0-1/2')
        api.get_item = MagicMock()
        collected = list(api.iter_tickets(updated_since='2024-01-01'))

    assert collected == tickets
    assert api.get.call_args.args[1] == 'Ticket'
    api.get_item.assert_not_called()


def test_iter_ticket_links_delegates_to_api_get():
    links = [{'id': 1, 'tickets_id_1': 10, 'tickets_id_2': 20}]

    with _patched_api() as api:
        api._api_get.return_value = iter(links)
        assert list(api.iter_ticket_links()) == links

    api._api_get.assert_called_once_with('Ticket_Ticket', expand_dropdowns='false')
