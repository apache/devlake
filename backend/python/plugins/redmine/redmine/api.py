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

from typing import Iterator, Optional, Union

from pydevlake.api import API, Paginator, PagedResponse, Request, request_hook


_PAGE_SIZE = 100
_ISSUE_SORT_UPDATED_ON = 'updated_on:desc'


class RedminePaginator(Paginator):
    def get_items(self, response) -> list:
        for value in response.json.values():
            if isinstance(value, list):
                return value
        return []

    def get_next_page_id(self, response) -> Optional[int]:
        data = response.json
        items = self.get_items(response)
        total = data.get('total_count', 0)
        offset = data.get('offset', 0)
        next_offset = offset + len(items)
        return next_offset if next_offset < total else None

    def set_next_page_param(self, request, next_page_id):
        request.query_args['offset'] = next_page_id


class RedmineAPI(API):
    paginator = RedminePaginator()

    @property
    def base_url(self) -> str:
        return str(self.connection.endpoint).rstrip('/')

    def __enter__(self):
        return self

    def __exit__(self, *_):
        pass

    @request_hook
    def authenticate(self, request: Request):
        request.headers['X-Redmine-API-Key'] = self.connection.token.get_secret_value()

    def test_connection(self):
        self.get('projects.json', limit=1)

    def projects(self):
        return self.get('projects.json', limit=_PAGE_SIZE)

    def issues(
        self,
        project_id: str,
        include_journals: bool = False,
        include_relations: bool = False,
        created_on: Optional[str] = None,
        updated_since: Optional[str] = None,
        limit: Optional[int] = None,
    ) -> Iterator[dict]:
        yield from self.iter_issues(
            project_id,
            include_journals=include_journals,
            include_relations=include_relations,
            created_on=created_on,
            updated_since=updated_since,
            limit=limit,
        )

    def iter_issues(
        self,
        project_id: str,
        include_journals: bool = False,
        include_relations: bool = False,
        created_on: Optional[str] = None,
        updated_since: Optional[str] = None,
        limit: Optional[int] = None,
    ) -> Iterator[dict]:
        params = self._issue_params(
            project_id,
            include_journals=include_journals,
            include_relations=include_relations,
            created_on=created_on,
        )
        if updated_since:
            yield from self._iter_issues(
                params,
                limit=limit,
                stop_before_updated_on=updated_since,
                sort=_ISSUE_SORT_UPDATED_ON,
            )
            return
        if limit is not None:
            yield from self._iter_issues(params, limit=limit)
            return
        yield from self.get('issues.json', **params)

    def issue(self, issue_id: Union[int, str], include_journals: bool = False) -> dict:
        params = {}
        if include_journals:
            params['include'] = 'journals'
        response = self.get(f'issues/{issue_id}.json', **params)
        return response.json['issue']

    @staticmethod
    def _issue_params(
        project_id: str,
        include_journals: bool = False,
        include_relations: bool = False,
        created_on: Optional[str] = None,
    ) -> dict:
        params = {'project_id': project_id, 'status_id': '*', 'limit': _PAGE_SIZE}
        includes = []
        if include_journals:
            includes.append('journals')
        if include_relations:
            includes.append('relations')
        if includes:
            params['include'] = ','.join(includes)
        if created_on:
            params['created_on'] = created_on
        return params

    @staticmethod
    def _should_stop_issue(issue: dict, stop_before_updated_on: Optional[str]) -> bool:
        if stop_before_updated_on:
            updated_on = issue.get('updated_on')
            if updated_on and updated_on <= stop_before_updated_on:
                return True
        return False

    def _iter_issues(
        self,
        params: dict,
        limit: Optional[int] = None,
        stop_before_updated_on: Optional[str] = None,
        sort: Optional[str] = None,
    ) -> Iterator[dict]:
        query = dict(params)
        if sort:
            query['sort'] = sort
        yielded = 0
        offset = 0
        while limit is None or yielded < limit:
            query['limit'] = _PAGE_SIZE if limit is None else min(_PAGE_SIZE, limit - yielded)
            if offset:
                query['offset'] = offset
            elif 'offset' in query:
                del query['offset']
            paged = self.get('issues.json', **query)
            items = paged.items or []
            if not items:
                break
            for item in items:
                if self._should_stop_issue(item, stop_before_updated_on):
                    return
                yield item
                yielded += 1
                if limit is not None and yielded >= limit:
                    return
            if paged.next_page_request is None:
                break
            offset += len(items)

    def time_entries(self, project_id: str):
        return self.get('time_entries.json', project_id=project_id, limit=_PAGE_SIZE)
