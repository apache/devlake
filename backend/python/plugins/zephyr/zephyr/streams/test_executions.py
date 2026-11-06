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

from typing import Iterable, Optional
import datetime

from zephyr.api import ProjectLookups, ZephyrAPI
from zephyr.models import Project, TestCase, TestExecution
from zephyr.utils import (
    parse_datetime,
    resource_id,
    lookup_name,
    json_dumps,
    extract_issue_links,
    scope_project_key,
    attach_x_request,
)
import pydevlake.domain_layer.qa as qa
from pydevlake import Stream, DomainType, domain_id
from pydevlake.collection_time import collection_time_after, format_rfc3339_utc


def _track_max_date(state: dict, key: str, value: Optional[str]) -> None:
    if not value:
        return
    current = state.get(key)
    if current is None or value > current:
        state[key] = value


def _execution_start_time(
    finish: Optional[datetime.datetime],
    execution_time_ms: Optional[int],
) -> Optional[datetime.datetime]:
    if finish is None:
        return None
    if not execution_time_ms or execution_time_ms < 0:
        return finish
    return finish - datetime.timedelta(milliseconds=execution_time_ms)


class TestExecutions(Stream):
    tool_model = TestExecution
    domain_types = [DomainType.CODE_QUALITY]
    domain_models = [qa.QaTestCaseExecution]

    def collect(self, state, context) -> Iterable[tuple[object, dict]]:
        project: Project = context.scope
        project_key = scope_project_key(project)
        run_state = dict(state)
        actual_end_after = None
        if context.incremental:
            actual_end_after = run_state.get('actual_end_date_after')
        else:
            time_after = collection_time_after(context)
            if time_after:
                actual_end_after = format_rfc3339_utc(time_after)

        with ZephyrAPI(context.connection) as api:
            lookups = api.project_lookups(project_key)
            for raw in api.iter_test_executions_nextgen(
                project_key,
                actual_end_date_after=actual_end_after,
            ):
                enriched = self._enrich_test_execution(raw, lookups, project_key)
                _track_max_date(run_state, 'actual_end_date_after', enriched.get('actualEndDate'))
                attach_x_request(
                    enriched,
                    f'{api.base_url}/testexecutions/nextgen',
                    projectKey=project_key,
                    actualEndDateAfter=actual_end_after,
                )
                yield enriched, run_state

    def extract(self, raw: dict) -> TestExecution:
        return TestExecution(
            id=raw['id'],
            key=raw.get('key'),
            project_key=raw.get('projectKey'),
            test_case_id=raw.get('testCaseId'),
            test_cycle_id=raw.get('testCycleId'),
            test_execution_status_id=raw.get('testExecutionStatusId'),
            test_execution_status_name=raw.get('testExecutionStatusName'),
            jira_project_version_id=raw.get('jiraProjectVersionId'),
            jira_project_version_name=raw.get('jiraProjectVersionName'),
            actual_end_date=parse_datetime(raw.get('actualEndDate')),
            execution_time=raw.get('executionTime'),
            estimated_time=raw.get('estimatedTime'),
            executed_by_id=raw.get('executedById'),
            assigned_to_id=raw.get('assignedToId'),
            comment=raw.get('comment'),
            automated=raw.get('automated'),
            custom_fields_json=raw.get('customFieldsJson'),
            issue_links_json=raw.get('issueLinksJson'),
        )

    def convert(self, execution: TestExecution, ctx) -> Iterable[qa.QaTestCaseExecution]:
        if not execution.test_case_id:
            return
        test_case_id = domain_id(TestCase, ctx.connection.id, execution.test_case_id)
        finish_time = execution.actual_end_date
        yield qa.QaTestCaseExecution(
            id=domain_id(TestExecution, ctx.connection.id, execution.id),
            qa_project_id=ctx.scope.domain_id(),
            qa_test_case_id=test_case_id,
            create_time=finish_time,
            start_time=_execution_start_time(finish_time, execution.execution_time),
            finish_time=finish_time,
            creator_id=execution.executed_by_id,
            status=self._map_execution_status(execution.test_execution_status_name),
        )

    @staticmethod
    def _map_execution_status(status_name: Optional[str]) -> str:
        if not status_name:
            return 'PENDING'
        normalized = status_name.strip().lower()
        if normalized in ('pass', 'passed'):
            return 'SUCCESS'
        elif normalized in ('fail', 'failed', 'blocked'):
            return 'FAILED'
        elif normalized in ('in progress', 'in_progress', 'executing'):
            return 'IN_PROGRESS'
        else:
            return 'PENDING'

    @staticmethod
    def _enrich_test_execution(raw: dict, lookups: ProjectLookups, project_key: str) -> dict:
        test_case = raw.get('testCase') or {}
        test_cycle = raw.get('testCycle') or {}
        status = raw.get('testExecutionStatus') or {}
        version = raw.get('jiraProjectVersion') or {}
        links = raw.get('links')
        custom_fields = raw.get('customFields')
        return {
            **raw,
            'projectKey': project_key,
            'testCaseId': resource_id(test_case),
            'testCycleId': resource_id(test_cycle),
            'testExecutionStatusId': resource_id(status),
            'testExecutionStatusName': lookup_name(lookups, 'execution_statuses', status),
            'jiraProjectVersionId': resource_id(version),
            'jiraProjectVersionName': version.get('name') if version else None,
            # nextgen returns flat ids; fall back to nested user refs if present
            'executedById': raw.get('executedById') or (raw.get('executedBy') or {}).get('accountId'),
            'assignedToId': raw.get('assignedToId') or (raw.get('assignedTo') or {}).get('accountId'),
            'customFieldsJson': json_dumps(custom_fields),
            'issueLinksJson': json_dumps(extract_issue_links(links)),
        }
