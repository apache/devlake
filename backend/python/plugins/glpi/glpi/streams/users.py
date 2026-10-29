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
from glpi.collection import prepare_on_demand_raw, referenced_user_ids
from glpi.models import User
from glpi.parsing import on_demand_id_from_url
from glpi.timezone import utc_now
import pydevlake.domain_layer.crossdomain as crossdomain
from pydevlake import Stream, DomainType
from pydevlake.context import Context
from pydevlake.extractor import autoextract
from pydevlake.subtasks import Collector
from sqlmodel import Session


class OnDemandUsersCollector(Collector):

    def fetch(self, state, session: Session, ctx: Context):
        from glpi.main import GlpiPlugin

        user_ids = referenced_user_ids(session, GlpiPlugin(), ctx)
        if not user_ids:
            return
        with GlpiAPI(ctx.connection) as api:
            for user_id in sorted(user_ids):
                if user_id <= 0:
                    continue
                prepared = prepare_on_demand_raw(
                    api.get_item('User', user_id), user_id, 'User', api.base_url,
                )
                if prepared is None:
                    continue
                yield prepared, state


class Users(Stream):
    tool_model = User
    domain_types = [DomainType.CROSS, DomainType.TICKET]
    domain_models = [crossdomain.Account]

    def __init__(self, plugin_name: str):
        super().__init__(plugin_name)
        self.collector = OnDemandUsersCollector(self)

    def collect(self, state, context) -> Iterable[tuple[object, dict]]:
        yield from ()

    def extract(self, raw: dict) -> User:
        raw = dict(raw)
        item_id = on_demand_id_from_url(raw, 'User')
        if item_id is not None:
            raw.setdefault('id', item_id)
        return autoextract(raw, self.tool_model)

    def convert(self, user: User, ctx) -> Iterable[object]:
        full_name = ' '.join(
            part for part in [user.firstname, user.realname] if part
        ).strip() or user.name or ''
        yield crossdomain.Account(
            email=user.email or '',
            full_name=full_name,
            user_name=user.name or str(user.id),
            avatar_url='',
            organization='',
            created_date=utc_now(),
            status=1,
        )
