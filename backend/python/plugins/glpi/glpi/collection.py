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
from typing import Optional, Union

from pydevlake.model import raw_data_params
from sqlmodel import Session

from glpi.parsing import parse_glpi_dropdown

_USER_RAW_STREAMS = (
    'tickets',
    'ticketusers',
    'ticketfollowups',
    'tickettasks',
    'ticketlogs',
)


def scan_raw_rows(session: Session, plugin, stream_name: str, params: str):
    stream = plugin.get_stream(stream_name)
    raw_model = stream.raw_model(session)
    return session.query(raw_model).where(raw_model.params == params).yield_per(500)


def prepare_on_demand_raw(
    raw: dict,
    item_id: int,
    resource: str,
    base_url: str,
) -> Optional[dict]:
    if item_id <= 0 or not raw:
        return None
    raw.setdefault('id', item_id)
    raw['x_request_url'] = f'{base_url}/apirest.php/{resource}/{item_id}'
    return raw


def referenced_user_ids(session: Session, plugin, ctx) -> set[int]:
    user_ids: set[int] = set()
    params = raw_data_params(ctx.connection.id, ctx.scope.id)
    for stream_name in _USER_RAW_STREAMS:
        for raw in scan_raw_rows(session, plugin, stream_name, params):
            data = json.loads(raw.data)
            if stream_name == 'tickets':
                _add_user_id(user_ids, data.get('users_id_recipient'))
                _add_user_id(user_ids, data.get('users_id_lastupdater'))
            elif stream_name == 'ticketusers':
                _add_user_id(user_ids, data.get('users_id'))
            elif stream_name == 'ticketfollowups':
                _add_user_id(user_ids, data.get('users_id'))
            elif stream_name == 'tickettasks':
                _add_user_id(user_ids, data.get('users_id'))
                _add_user_id(user_ids, data.get('users_id_tech'))
            elif stream_name == 'ticketlogs':
                _add_user_id(user_ids, data.get('user_name'))
                _add_user_id(user_ids, data.get('users_id'))
    return user_ids


def referenced_group_ids(session: Session, plugin, ctx) -> set[int]:
    group_ids: set[int] = set()
    params = raw_data_params(ctx.connection.id, ctx.scope.id)
    for raw in scan_raw_rows(session, plugin, 'ticketgroups', params):
        data = json.loads(raw.data)
        _, group_id = parse_glpi_dropdown(data.get('groups_id'))
        if group_id is not None:
            group_ids.add(group_id)
    return group_ids


def _add_user_id(user_ids: set[int], value: Optional[Union[str, int]]) -> None:
    _, user_id = parse_glpi_dropdown(value)
    if user_id is not None:
        user_ids.add(int(user_id))
