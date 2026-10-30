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

from zephyr.api import ProjectLookups, ZephyrAPI
from zephyr.models import Project, TestCycle
from zephyr.utils import (
    parse_datetime,
    resource_id,
    lookup_name,
    passes_time_filter,
    scope_project_key,
    attach_x_request,
)
from pydevlake import Stream, DomainType
from pydevlake.collection_time import collection_time_after


class TestCycles(Stream):
    tool_model = TestCycle
    domain_types = [DomainType.CODE_QUALITY]
    domain_models = []

    def collect(self, state, context) -> Iterable[tuple[object, dict]]:
        project: Project = context.scope
        project_key = scope_project_key(project)
        run_state = dict(state)
        # API has no date filter; apply project time frame client-side on full sync only.
        # Incremental recollects all so updates to existing cycles are kept.
        started_after = None if context.incremental else collection_time_after(context)
        with ZephyrAPI(context.connection) as api:
            lookups = api.project_lookups(project_key)
            for raw in api.iter_test_cycles_nextgen(project_key):
                if not passes_time_filter(raw.get('plannedStartDate'), started_after):
                    continue
                enriched = self._enrich_test_cycle(raw, lookups, project_key)
                request_input = {'projectKey': project_key}
                if started_after:
                    request_input['plannedStartAfter'] = started_after.isoformat()
                attach_x_request(enriched, f'{api.base_url}/testcycles/nextgen', **request_input)
                yield enriched, run_state

    def extract(self, raw: dict) -> TestCycle:
        return TestCycle(
            id=raw['id'],
            key=raw.get('key'),
            project_key=raw.get('projectKey'),
            name=raw.get('name'),
            description=raw.get('description'),
            folder_id=raw.get('folderId'),
            folder_name=raw.get('folderName'),
            status_id=raw.get('statusId'),
            status_name=raw.get('statusName'),
            jira_project_version_id=raw.get('jiraProjectVersionId'),
            jira_project_version_name=raw.get('jiraProjectVersionName'),
            planned_start_date=parse_datetime(raw.get('plannedStartDate')),
            planned_end_date=parse_datetime(raw.get('plannedEndDate')),
        )

    def convert(self, cycle: TestCycle, ctx):
        yield from ()

    @staticmethod
    def _enrich_test_cycle(raw: dict, lookups: ProjectLookups, project_key: str) -> dict:
        folder = raw.get('folder')
        status = raw.get('status')
        version = raw.get('jiraProjectVersion')
        return {
            **raw,
            'projectKey': project_key,
            'folderId': resource_id(folder),
            'folderName': lookup_name(lookups, 'folders', folder),
            'statusId': resource_id(status),
            'statusName': lookup_name(lookups, 'cycle_statuses', status),
            'jiraProjectVersionId': resource_id(version),
            'jiraProjectVersionName': version.get('name') if version else None,
        }
