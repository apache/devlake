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
from dataclasses import dataclass, field
from typing import Iterable, Optional, Union

from glpi.api import GlpiAPI
from glpi.models import Entity, Ticket, User
from glpi.parsing import parse_glpi_dropdown
from glpi.timezone import connection_timezone, glpi_search_time_from_iso
from pydevlake import Stream, DomainType
from pydevlake.collection_time import collection_time_after
from pydevlake.context import Context
from pydevlake.extractor import autoextract
from pydevlake.subtasks import Extractor
from sqlmodel import Session

_LOOKUP_KEY = '_glpi_lookup'


@dataclass
class GlpiLookup:
    users_by_name: dict[str, int] = field(default_factory=dict)

    @classmethod
    def build(cls, session: Session, connection_id: int) -> 'GlpiLookup':
        users_by_name: dict[str, int] = {}
        for row in session.query(User).where(User.connection_id == connection_id):
            if row.id is None or not row.name:
                continue
            users_by_name[str(row.name).strip().lower()] = int(row.id)
        return cls(users_by_name=users_by_name)


def _set_lookup(context: Context, lookup: GlpiLookup) -> None:
    context.options[_LOOKUP_KEY] = lookup


def _lookup(context: Context) -> GlpiLookup:
    stored = context.options.get(_LOOKUP_KEY)
    if stored is None:
        stored = GlpiLookup()
        _set_lookup(context, stored)
    return stored


class TicketExtractor(Extractor):

    def fetch(self, state, session: Session, ctx: Context):
        _set_lookup(ctx, GlpiLookup.build(session, ctx.connection.id))
        raw_model = self.stream.raw_model(session)
        params = self._params(ctx)
        last_id = 0
        batch_size = 200
        while True:
            batch = (
                session.query(raw_model)
                .where(raw_model.params == params)
                .where(raw_model.id > last_id)
                .order_by(raw_model.id)
                .limit(batch_size)
                .all()
            )
            if not batch:
                break
            for raw in batch:
                last_id = raw.id
                yield raw, state

    def process(self, raw, session: Session, ctx: Context):
        tool_model = self.stream.extract(json.loads(raw.data), ctx)
        tool_model.set_raw_origin(raw)
        tool_model.connection_id = ctx.connection.id
        tool_model.set_updated_at()
        session.merge(tool_model)


def _glpi_native_ticket_keys() -> set[str]:
        ticket_enrichment_fields = {
            'category_name', 'location_name', 'supplier_name', 'requesttype_name',
            'requester_name', 'assignee_id', 'assignee_name',
            'parent_ticket_id', 'solution_content', 'plugin_fields_json',
        }
        glpi_ticket_ignored_keys = {'links'}
        return {
            name for name, field_info in Ticket.__fields__.items()
            if name not in ticket_enrichment_fields
            and not field_info.field_info.extra.get('source')
        } | glpi_ticket_ignored_keys


_GLPI_NATIVE_TICKET_KEYS = _glpi_native_ticket_keys()


class Tickets(Stream):
    tool_model = Ticket
    domain_types = [DomainType.TICKET]
    domain_models = []

    def __init__(self, plugin_name: str):
        super().__init__(plugin_name)
        self.extractor = TicketExtractor(self)

    @property
    def subtasks(self):
        return [self.collector, self.extractor]

    def collect(self, state, context) -> Iterable[tuple[object, dict]]:
        entity: Entity = context.scope
        run_state = dict(state)
        use_incremental = context.options.get('incremental')
        updated_since = run_state.get('updated_since') if use_incremental else None
        created_after = None
        if not use_incremental:
            time_after = collection_time_after(context)
            if time_after:
                created_after = glpi_search_time_from_iso(
                    time_after.isoformat(),
                    connection_timezone(context.connection.timezone),
                )
        catalog_cache: dict[str, str] = {}
        user_cache: dict[int, str] = {}
        with GlpiAPI(context.connection) as api:
            for raw_ticket in api.iter_tickets(
                entity_id=entity.id,
                updated_since=updated_since,
                created_after=created_after,
            ):
                self._track_updated_since(run_state, raw_ticket.get('date_mod'))
                self._resolve_bare_catalog_fields(api, raw_ticket, catalog_cache)
                self._resolve_bare_user_fields(api, raw_ticket, user_cache)
                raw_ticket['x_request_url'] = f'{api.base_url}/apirest.php/Ticket'
                raw_ticket['x_request_input'] = {'EntityId': entity.id}
                yield raw_ticket, run_state

    @staticmethod
    def _track_updated_since(state: dict, date_mod: Optional[str]) -> None:
        if not date_mod:
            return
        current = state.get('updated_since')
        if current is None or date_mod > current:
            state['updated_since'] = date_mod

    @staticmethod
    def _resolve_bare_catalog_fields(
        api: GlpiAPI,
        ticket_raw: dict,
        catalog_cache: dict[str, str],
    ) -> None:
        catalog_fields = {
            'itilcategories_id': ('ITILCategory', ('completename', 'name')),
            'locations_id': ('Location', ('completename', 'name')),
            'suppliers_id': ('Supplier', ('name',)),
            'requesttypes_id': ('RequestType', ('name',)),
        }
        for field_name, (itemtype, name_fields) in catalog_fields.items():
            name, item_id = parse_glpi_dropdown(ticket_raw.get(field_name))
            if item_id is None or item_id <= 0 or name:
                continue
            cache_key = f'{itemtype}:{item_id}'
            if cache_key not in catalog_cache:
                raw = api.get_item(itemtype, item_id)
                resolved = next(
                    (raw.get(field) for field in name_fields if raw.get(field)),
                    None,
                )
                catalog_cache[cache_key] = resolved or str(item_id)
            ticket_raw[field_name] = f'{catalog_cache[cache_key]} ({item_id})'

    @staticmethod
    def _resolve_bare_user_fields(
        api: GlpiAPI,
        ticket_raw: dict,
        user_cache: dict[int, str],
    ) -> None:
        for field in ('users_id_recipient', 'users_id_lastupdater'):
            user_id = Tickets._bare_dropdown_id(ticket_raw.get(field))
            if user_id is None:
                continue
            if user_id not in user_cache:
                user_cache[user_id] = Tickets._user_display_name(api.get_item('User', user_id))
            ticket_raw[field] = f'{user_cache[user_id]} ({user_id})'

    @staticmethod
    def _bare_dropdown_id(value) -> Optional[int]:
        name, item_id = parse_glpi_dropdown(value)
        return item_id if item_id is not None and not name else None

    @staticmethod
    def _user_display_name(user_raw: dict) -> str:
        for field in ('realname', 'firstname', 'name'):
            value = user_raw.get(field)
            if value:
                text = str(value).strip()
                if text:
                    return text
        return str(user_raw.get('id', ''))

    def extract(self, raw: dict, ctx: Optional[Context] = None) -> Ticket:
        ticket_model = autoextract(raw, self.tool_model)
        if ctx is not None:
            self._apply_ticket_enrichment(ticket_model, raw, ctx)
        return ticket_model

    @staticmethod
    def _apply_ticket_enrichment(
        ticket_model: Ticket,
        raw: dict,
        ctx: Context,
    ) -> None:
        requester_name, requester_id = Tickets._resolve_glpi_user_ref(ctx, raw.get('users_id_recipient'))
        ticket_model.requester_name = requester_name
        if requester_id is not None:
            ticket_model.users_id_recipient = requester_id

        entity_name, entity_id = parse_glpi_dropdown(raw.get('entities_id'))
        if entity_id is not None and str(entity_id) == str(ctx.scope.id):
            ticket_model.entities_id = ctx.scope.completename or ctx.scope.name
        elif entity_name and not entity_id:
            ticket_model.entities_id = entity_name

        category_name, category_id = parse_glpi_dropdown(raw.get('itilcategories_id'))
        ticket_model.itilcategories_id = category_id
        ticket_model.category_name = category_name

        location_name, location_id = parse_glpi_dropdown(raw.get('locations_id'))
        ticket_model.locations_id = location_id
        ticket_model.location_name = location_name

        supplier_name, supplier_id = parse_glpi_dropdown(raw.get('suppliers_id'))
        ticket_model.suppliers_id = supplier_id
        ticket_model.supplier_name = supplier_name

        requesttype_name, requesttype_id = parse_glpi_dropdown(raw.get('requesttypes_id'))
        ticket_model.requesttypes_id = requesttype_id
        ticket_model.requesttype_name = requesttype_name

        plugin_fields = Tickets._extract_plugin_fields(raw)
        if plugin_fields:
            ticket_model.plugin_fields_json = json.dumps(plugin_fields)

    @staticmethod
    def _resolve_glpi_user_ref(
        context: Context,
        value: Optional[Union[str, int]],
    ) -> tuple[Optional[str], Optional[int]]:
        name, user_id = parse_glpi_dropdown(value)
        if user_id is not None:
            return name, user_id
        if not name:
            return None, None
        resolved_id = _lookup(context).users_by_name.get(name.strip().lower())
        if resolved_id is not None:
            return name, resolved_id
        return name, None

    @staticmethod
    def _extract_plugin_fields(raw: dict) -> dict[str, str]:
        return {
            key: str(value)
            for key, value in raw.items()
            if (
                key not in _GLPI_NATIVE_TICKET_KEYS
                and not key.startswith('_')
                and not key.startswith('x_')
                and value is not None
                and value != ''
            )
        }
