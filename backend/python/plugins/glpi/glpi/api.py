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

import re
from typing import Optional

import requests as req

from pydevlake.api import API, APIException, Paginator, Request, request_hook
from pydevlake.logger import logger


_API_PATH = 'apirest.php'
_PAGE_SIZE = 100

# GLPI Ticket table columns used for list sorting (not search-option IDs)
_TICKET_SORT_OPENING_DATE = 'date'
_TICKET_SORT_DATE_MOD = 'date_mod'


class GlpiPaginator(Paginator):
    def get_items(self, response) -> list:
        return response.json

    def get_next_page_id(self, response) -> Optional[int]:
        content_range = response.headers.get('Content-Range', '')
        match = re.match(r'(\d+)-(\d+)/(\d+)', content_range)
        if not match:
            return None
        end, total = int(match.group(2)), int(match.group(3))
        next_start = end + 1
        return next_start if next_start < total else None

    def set_next_page_param(self, request, next_page_id):
        request.query_args['range'] = f'{next_page_id}-{next_page_id + _PAGE_SIZE - 1}'


class GlpiAPI(API):
    paginator = GlpiPaginator()

    def __init__(self, connection):
        super().__init__(connection)
        self._session_token = None

    @property
    def base_url(self) -> str:
        return str(self.connection.endpoint).rstrip('/')

    def test_connection(self):
        self._init_session()

    def _init_session(self):
        resp = req.get(
            f'{self.base_url}/{_API_PATH}/initSession',
            headers={
                'App-Token': self.connection.token.get_secret_value(),
                'Authorization': f'user_token {self.connection.secret_key.get_secret_value()}',
            }
        )
        resp.raise_for_status()
        self._session_token = resp.json()['session_token']

    @request_hook
    def authenticate(self, request: Request):
        if self._session_token is None:
            self._init_session()
        request.headers['App-Token'] = self.connection.token.get_secret_value()
        request.headers['Session-Token'] = self._session_token

    def __enter__(self):
        return self

    def __exit__(self, *_):
        if self._session_token is None:
            return
        try:
            self.get(_API_PATH, 'killSession')
        except APIException:
            logger.warning("Failed to kill GLPI session", exc_info=True)
        finally:
            self._session_token = None

    def entities(self, expand_dropdowns='true'):
        return self._api_get('Entity', expand_dropdowns=expand_dropdowns)

    def _api_get(self, resource, *path_args, expand_dropdowns='false', get_hateoas='false', **kwargs):
        return self.get(
            _API_PATH, resource, *path_args,
            range=f'0-{_PAGE_SIZE - 1}',
            expand_dropdowns=expand_dropdowns,
            get_hateoas=get_hateoas,
            **kwargs,
        )

    def iter_tickets(
        self,
        entity_id: Optional[str] = None,
        updated_since: Optional[str] = None,
        created_after: Optional[str] = None,
        limit: Optional[int] = None,
    ):
        if updated_since:
            yield from self._iter_ticket_list(
                entity_id=entity_id,
                limit=limit,
                stop_before_date_mod=updated_since,
                sort=_TICKET_SORT_DATE_MOD,
            )
            return
        if created_after:
            yield from self._iter_ticket_list(
                entity_id=entity_id,
                limit=limit,
                stop_before_opening_date=created_after,
                sort=_TICKET_SORT_OPENING_DATE,
            )
            return
        yield from self._iter_ticket_list(entity_id=entity_id, limit=limit)

    @staticmethod
    def _should_stop_ticket_page(
        ticket: dict,
        stop_before_date_mod: Optional[str],
        stop_before_opening_date: Optional[str],
    ) -> bool:
        if stop_before_date_mod:
            date_mod = ticket.get('date_mod')
            if date_mod and date_mod <= stop_before_date_mod:
                return True
        if stop_before_opening_date:
            opening_date = ticket.get('date')
            if opening_date and opening_date <= stop_before_opening_date:
                return True
        return False

    def _ticket_list_query(
        self,
        entity_id: Optional[str],
        sort: Optional[str] = None,
    ) -> dict:
        query = dict(expand_dropdowns='true', get_hateoas='false')
        if sort is not None:
            query['sort'] = sort
            query['order'] = 'DESC'
        if entity_id and entity_id != '0':
            query['searchText[entities_id]'] = entity_id
        return query

    def _iter_ticket_list(
        self,
        entity_id: Optional[str] = None,
        limit: Optional[int] = None,
        stop_before_date_mod: Optional[str] = None,
        stop_before_opening_date: Optional[str] = None,
        sort: Optional[str] = None,
    ):
        query = self._ticket_list_query(entity_id, sort)
        has_date_stop = stop_before_date_mod or stop_before_opening_date

        if limit is None and not has_date_stop:
            yield from self.get(
                _API_PATH, 'Ticket',
                range=f'0-{_PAGE_SIZE - 1}',
                **query,
            )
            return

        yielded = 0
        range_start = 0
        while limit is None or yielded < limit:
            page_size = _PAGE_SIZE if limit is None else min(_PAGE_SIZE, limit - yielded)
            response = self.get(
                _API_PATH, 'Ticket',
                range=f'{range_start}-{range_start + page_size - 1}',
                **query,
            )
            items = self.paginator.get_items(response)
            if not items:
                break
            if not isinstance(items, list):
                items = [items]
            for item in items:
                if self._should_stop_ticket_page(
                    item, stop_before_date_mod, stop_before_opening_date,
                ):
                    return
                yield item
                yielded += 1
                if limit is not None and yielded >= limit:
                    return

            content_range = response.headers.get('Content-Range', '')
            match = re.match(r'(\d+)-(\d+)/(\d+)', content_range)
            if not match:
                break
            end, total = int(match.group(2)), int(match.group(3))
            if end + 1 >= total:
                break
            range_start = end + 1

    @staticmethod
    def _item_body(body) -> dict:
        if isinstance(body, dict):
            return body
        if isinstance(body, list):
            for entry in body:
                if isinstance(entry, dict):
                    return entry
        return {}

    def get_item(self, itemtype: str, item_id: int) -> dict:
        if item_id <= 0:
            return {}
        try:
            response = self.get(
                _API_PATH, itemtype, str(item_id),
                expand_dropdowns='false', get_hateoas='false',
            )
            return self._item_body(response.json)
        except APIException as exc:
            if self._is_item_not_found(exc):
                return {}
            raise

    @staticmethod
    def _is_item_not_found(exc: APIException) -> bool:
        response = exc.response
        if response.status != 404:
            return False
        body = response.body
        if isinstance(body, bytes):
            body = body.decode('utf-8', errors='replace')
        text = str(body)
        return 'ERROR_ITEM_NOT_FOUND' in text or 'Item not found' in text

    def iter_items(self, itemtype: str, expand_dropdowns='true'):
        return self._api_get(itemtype, expand_dropdowns=expand_dropdowns)

    def ticket_subitems(self, ticket_id: int, sub_type: str):
        return self._api_get(
            'Ticket', str(ticket_id), sub_type, expand_dropdowns='false',
        )

    def iter_ticket_links(self):
        yield from self.iter_items('Ticket_Ticket', expand_dropdowns='false')
