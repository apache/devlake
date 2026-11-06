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
from typing import Iterable, Optional

from zephyr.api import ProjectLookups, ZephyrAPI
from zephyr.models import Project, ScopeConfig, TestCase
from zephyr.utils import (
    parse_datetime,
    extract_issue_links,
    resource_id,
    lookup_name,
    json_dumps,
    passes_time_filter,
    parse_json_object,
    scope_project_key,
    attach_x_request,
)
import pydevlake.domain_layer.qa as qa
from pydevlake import Stream, DomainType, domain_id
from pydevlake.collection_time import collection_time_after


class TestCases(Stream):
    tool_model = TestCase
    domain_types = [DomainType.CODE_QUALITY]
    domain_models = [qa.QaTestCase]

    def collect(self, state, context) -> Iterable[tuple[object, dict]]:
        project: Project = context.scope
        project_key = scope_project_key(project)
        run_state = dict(state)
        # API has no date filter or updatedOn; apply project time frame client-side on
        # full sync only. Incremental recollects all so updates to existing cases are kept.
        created_after = None if context.incremental else collection_time_after(context)
        with ZephyrAPI(context.connection) as api:
            lookups = api.project_lookups(project_key)
            for raw in api.iter_test_cases_nextgen(project_key):
                if not passes_time_filter(raw.get('createdOn'), created_after):
                    continue
                enriched = self._enrich_test_case(raw, lookups, project_key)
                request_input = {'projectKey': project_key}
                if created_after:
                    request_input['createdAfter'] = created_after.isoformat()
                attach_x_request(enriched, f'{api.base_url}/testcases/nextgen', **request_input)
                yield enriched, run_state

    def extract(self, raw: dict) -> TestCase:
        return TestCase(
            id=raw['id'],
            key=raw.get('key'),
            project_key=raw.get('projectKey'),
            name=raw.get('name'),
            objective=raw.get('objective'),
            precondition=raw.get('precondition'),
            created_on=parse_datetime(raw.get('createdOn')),
            folder_id=raw.get('folderId'),
            folder_name=raw.get('folderName'),
            component_id=raw.get('componentId'),
            component_name=raw.get('componentName'),
            priority_id=raw.get('priorityId'),
            priority_name=raw.get('priorityName'),
            status_id=raw.get('statusId'),
            status_name=raw.get('statusName'),
            owner_account_id=raw.get('ownerAccountId'),
            estimated_time=raw.get('estimatedTime'),
            labels_json=raw.get('labelsJson'),
            custom_fields_json=raw.get('customFieldsJson'),
            issue_links_json=raw.get('issueLinksJson'),
        )

    def convert(self, test_case: TestCase, ctx) -> Iterable[qa.QaTestCase]:
        scope_config: ScopeConfig = ctx.scope_config
        custom_fields = parse_json_object(test_case.custom_fields_json)
        test_type = self._custom_field_value(
            custom_fields,
            scope_config.test_type_custom_field,
        )
        yield qa.QaTestCase(
            id=domain_id(TestCase, ctx.connection.id, test_case.id),
            name=test_case.name or test_case.key or '',
            type=test_type or 'functional',
            create_time=test_case.created_on,
            creator_id=test_case.owner_account_id,
            qa_api_id=test_case.key,
            qa_project_id=ctx.scope.domain_id(),
        )

    @staticmethod
    def _custom_field_value(custom_fields: Optional[dict], field_name: str) -> Optional[str]:
        if not custom_fields or not field_name:
            return None
        value = custom_fields.get(field_name)
        if value is None:
            return None
        elif isinstance(value, (list, dict)):
            return json.dumps(value)
        else:
            return str(value)

    @staticmethod
    def _enrich_test_case(raw: dict, lookups: ProjectLookups, project_key: str) -> dict:
        folder = raw.get('folder')
        component = raw.get('component')
        priority = raw.get('priority')
        status = raw.get('status')
        owner = raw.get('owner') or {}
        custom_fields = raw.get('customFields')
        labels = raw.get('labels') or []
        links = raw.get('links')
        return {
            **raw,
            'projectKey': project_key,
            'folderId': resource_id(folder),
            'folderName': lookup_name(lookups, 'folders', folder),
            'componentId': resource_id(component),
            'componentName': lookup_name(lookups, 'components', component),
            'priorityId': resource_id(priority),
            'priorityName': lookup_name(lookups, 'priorities', priority),
            'statusId': resource_id(status),
            'statusName': lookup_name(lookups, 'statuses', status),
            'ownerAccountId': owner.get('accountId'),
            'labelsJson': json_dumps(labels),
            'customFieldsJson': json_dumps(custom_fields),
            'issueLinksJson': json_dumps(extract_issue_links(links)),
        }
