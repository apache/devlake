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

from typing import Iterable

from glpi.api import GlpiAPI
from glpi.models import Connection, Entity, ScopeConfig
from glpi.timezone import validate_timezone_name
from glpi.streams.groups import Groups
from glpi.streams.ticket_domain import TicketDomain
from glpi.streams.ticket_substreams import (
    TicketFollowups,
    TicketGroups,
    TicketLinks,
    TicketLogs,
    TicketSolutions,
    TicketTasks,
    TicketUsers,
)
from glpi.streams.tickets import Tickets
from glpi.streams.users import Users

from pydevlake import Plugin, RemoteScopeGroup, TestConnectionResult
from pydevlake.logger import logger
from pydevlake.model import raw_data_params
import pydevlake.domain_layer.ticket as ticket

ROOT_GROUP_ID = 'root'


class GlpiPlugin(Plugin):

    @property
    def connection_type(self):
        return Connection

    @property
    def tool_scope_type(self):
        return Entity

    @property
    def scope_config_type(self):
        return ScopeConfig

    def domain_scopes(self, entity: Entity):
        yield ticket.Board(
            name=entity.completename or entity.name,
            description=entity.completename,
            type='ITSM',
        )

    def remote_scope_groups(self, connection) -> list[RemoteScopeGroup]:
        return [RemoteScopeGroup(id=ROOT_GROUP_ID, name='Entities')]

    def remote_scopes(self, connection, group_id: str) -> list[Entity]:
        with GlpiAPI(connection) as api:
            yield from self._entities(api, connection)

    def _entities(self, api: GlpiAPI, connection: Connection) -> Iterable[Entity]:
        raw_table = self._raw_scope_table_name()
        for raw_entity in sorted(
            api.entities(expand_dropdowns='false'),
            key=lambda raw: (raw.get('completename') or raw.get('name') or str(raw['id'])).lower(),
        ):
            entity_id = str(raw_entity['id'])
            entity = Entity(
                id=entity_id,
                name=raw_entity.get('name', ''),
                completename=raw_entity.get('completename', raw_entity.get('name', '')),
            )
            entity.connection_id = connection.id
            entity.raw_data_params = raw_data_params(connection.id, entity_id)
            entity.raw_data_table = raw_table
            yield entity

    def test_connection(self, connection: Connection) -> TestConnectionResult:
        try:
            validate_timezone_name(connection.timezone)
        except ValueError as e:
            return TestConnectionResult(success=False, message=str(e), status=400)
        with GlpiAPI(connection) as api:
            try:
                api.test_connection()
                return TestConnectionResult(success=True, message='Connection successful', status=200)
            except Exception as e:
                logger.warning("GLPI test_connection failed", exc_info=True)
                return TestConnectionResult(success=False, message=str(e), status=400)

    @property
    def streams(self):
        return [
            Tickets,
            TicketUsers,
            TicketGroups,
            TicketFollowups,
            TicketTasks,
            TicketLogs,
            TicketLinks,
            TicketSolutions,
            TicketDomain,
            Users,
            Groups,
        ]


if __name__ == '__main__':
    GlpiPlugin.start()
