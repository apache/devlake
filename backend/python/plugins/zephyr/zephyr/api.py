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

from typing import Iterator, Optional

from pydevlake.api import API, Paginator, Request, request_hook

_PAGE_SIZE = 1000


class ZephyrOffsetPaginator(Paginator):
    def get_items(self, response) -> list:
        return response.json.get('values') or []

    def get_next_page_id(self, response) -> Optional[int]:
        if response.json.get('isLast', True):
            return None
        start_at = response.json.get('startAt', 0)
        max_results = response.json.get('maxResults') or len(self.get_items(response))
        return start_at + max_results

    def set_next_page_param(self, request, next_page_id):
        request.query_args['startAt'] = str(next_page_id)


class ProjectLookups:
    def __init__(self):
        self.folders: dict[int, dict] = {}
        self.priorities: dict[int, dict] = {}
        self.statuses: dict[int, dict] = {}
        self.components: dict[int, dict] = {}
        self.execution_statuses: dict[int, dict] = {}
        self.cycle_statuses: dict[int, dict] = {}


class ZephyrAPI(API):
    paginator = ZephyrOffsetPaginator()

    @property
    def base_url(self) -> str:
        return str(self.connection.endpoint).rstrip('/')

    def __enter__(self):
        return self

    def __exit__(self, *_):
        pass

    @request_hook
    def authenticate(self, request: Request):
        request.headers['Authorization'] = (
            f'Bearer {self.connection.token.get_secret_value()}'
        )

    def test_connection(self):
        self.get('projects', maxResults=1, startAt=0)

    def projects(self) -> Iterator[dict]:
        yield from self.get('projects', maxResults=_PAGE_SIZE, startAt=0)

    def project_lookups(self, project_key: str) -> ProjectLookups:
        lookups = ProjectLookups()
        for folder in self.folders(project_key):
            lookups.folders[folder['id']] = folder
        for priority in self.priorities(project_key):
            lookups.priorities[priority['id']] = priority
        for status in self.statuses(project_key, 'TEST_CASE'):
            lookups.statuses[status['id']] = status
        for status in self.statuses(project_key, 'TEST_EXECUTION'):
            lookups.execution_statuses[status['id']] = status
        for status in self.statuses(project_key, 'TEST_CYCLE'):
            lookups.cycle_statuses[status['id']] = status
        # Components are Jira entities; Zephyr Scale Cloud API has no /components
        # endpoint. component_id is kept from the test-case ref; name stays unset.
        return lookups

    def folders(self, project_key: str, folder_type: Optional[str] = None) -> Iterator[dict]:
        params = {'projectKey': project_key, 'maxResults': _PAGE_SIZE, 'startAt': 0}
        if folder_type:
            params['folderType'] = folder_type
        yield from self.get('folders', **params)

    def priorities(self, project_key: str) -> Iterator[dict]:
        yield from self.get(
            'priorities',
            projectKey=project_key,
            maxResults=_PAGE_SIZE,
            startAt=0,
        )

    def statuses(self, project_key: str, status_type: str) -> Iterator[dict]:
        yield from self.get(
            'statuses',
            projectKey=project_key,
            statusType=status_type,
            maxResults=_PAGE_SIZE,
            startAt=0,
        )

    def iter_test_cases_nextgen(self, project_key: str) -> Iterator[dict]:
        yield from self._iter_nextgen('testcases/nextgen', projectKey=project_key)

    def iter_test_cycles_nextgen(self, project_key: str) -> Iterator[dict]:
        yield from self._iter_nextgen('testcycles/nextgen', projectKey=project_key)

    def iter_test_executions_nextgen(
        self,
        project_key: str,
        actual_end_date_after: Optional[str] = None,
    ) -> Iterator[dict]:
        params = {'projectKey': project_key, 'includeStepLinks': 'true'}
        if actual_end_date_after:
            params['actualEndDateAfter'] = actual_end_date_after
        yield from self._iter_nextgen('testexecutions/nextgen', **params)

    def _iter_nextgen(self, path: str, **query) -> Iterator[dict]:
        start_at_id = 0
        while True:
            response = self.get(
                path,
                limit=_PAGE_SIZE,
                startAtId=start_at_id,
                **query,
            )
            payload = response.json
            values = payload.get('values') or []
            for item in values:
                yield item
            if not values:
                return
            next_start = payload.get('nextStartAtId')
            if next_start is None or next_start == start_at_id:
                return
            start_at_id = next_start
