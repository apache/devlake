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
from glpi.collection import prepare_on_demand_raw, referenced_group_ids
from glpi.models import Group
from glpi.parsing import on_demand_id_from_url
from glpi.timezone import utc_now
import pydevlake.domain_layer.crossdomain as crossdomain
from pydevlake import Stream, DomainType
from pydevlake.context import Context
from pydevlake.extractor import autoextract
from pydevlake.subtasks import Collector
from sqlmodel import Session


class OnDemandGroupsCollector(Collector):

    def fetch(self, state, session: Session, ctx: Context):
        from glpi.main import GlpiPlugin

        group_ids = referenced_group_ids(session, GlpiPlugin(), ctx)
        if not group_ids:
            return
        with GlpiAPI(ctx.connection) as api:
            for group_id in sorted(group_ids):
                if group_id <= 0:
                    continue
                prepared = prepare_on_demand_raw(
                    api.get_item('Group', group_id), group_id, 'Group', api.base_url,
                )
                if prepared is None:
                    continue
                yield prepared, state


class Groups(Stream):
    tool_model = Group
    domain_types = [DomainType.CROSS, DomainType.TICKET]
    domain_models = [crossdomain.Account]

    def __init__(self, plugin_name: str):
        super().__init__(plugin_name)
        self.collector = OnDemandGroupsCollector(self)

    def collect(self, state, context) -> Iterable[tuple[object, dict]]:
        yield from ()

    def extract(self, raw: dict) -> Group:
        raw = dict(raw)
        item_id = on_demand_id_from_url(raw, 'Group')
        if item_id is not None:
            raw.setdefault('id', item_id)
        return autoextract(raw, self.tool_model)

    def convert(self, group: Group, ctx) -> Iterable[object]:
        display_name = group.completename or group.name or str(group.id)
        yield crossdomain.Account(
            email=f'group-{group.id}@glpi.local',
            full_name=display_name,
            user_name=group.name or str(group.id),
            avatar_url='',
            organization='',
            created_date=utc_now(),
            status=1,
        )
