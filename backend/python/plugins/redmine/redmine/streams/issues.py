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

import datetime
import json
from typing import Iterable, Optional

from redmine.api import RedmineAPI
from redmine.models import Issue, Project, ScopeConfig, StatusMapping, TypeMapping
import pydevlake.domain_layer.ticket as ticket
from pydevlake import Stream, DomainType, domain_id
from pydevlake.collection_time import collection_time_after, format_collection_date

_DEFAULT_TYPE_BY_TRACKER = {
    'Bug': 'INCIDENT',
    'Feature': 'REQUIREMENT',
    'Support': 'TASK',
    'Task': 'TASK',
}

_DEFAULT_STATUS_BY_NAME = {
    'New': 'TODO',
    'In Progress': 'IN_PROGRESS',
    'Feedback': 'IN_PROGRESS',
    'Resolved': 'DONE',
    'Closed': 'DONE',
    'Rejected': 'DONE',
}


class Issues(Stream):
    tool_model = Issue
    domain_types = [DomainType.TICKET]
    domain_models = [
        ticket.Issue,
        ticket.BoardIssue,
        ticket.IssueAssignee,
        ticket.IssueLabel,
        ticket.IssueRelationship,
    ]

    def collect(self, state, context) -> Iterable[tuple[object, dict]]:
        project: Project = context.scope
        run_state = dict(state)
        use_incremental = context.options.get('incremental')
        updated_since = run_state.get('updated_since') if use_incremental else None
        created_on = None
        if not use_incremental:
            time_after = collection_time_after(context)
            if time_after:
                created_on = f'>={format_collection_date(time_after)}'
        with RedmineAPI(context.connection) as api:
            for raw in api.issues(
                project.id,
                created_on=created_on,
                updated_since=updated_since,
                include_relations=True,
            ):
                self._track_updated_since(run_state, raw.get('updated_on'))
                raw['x_request_url'] = f'{api.base_url}/issues.json'
                raw['x_request_input'] = {'project_id': project.id}
                if created_on:
                    raw['x_request_input']['created_on'] = created_on
                if updated_since:
                    raw['x_request_input']['updated_since'] = updated_since
                yield raw, run_state

    @staticmethod
    def _track_updated_since(state: dict, updated_on: Optional[str]) -> None:
        if not updated_on:
            return
        current = state.get('updated_since')
        if current is None or updated_on > current:
            state['updated_since'] = updated_on

    def extract(self, raw: dict) -> Issue:
        parent = raw.get('parent') or {}
        category = raw.get('category') or {}
        fixed_version = raw.get('fixed_version') or {}
        custom_fields = raw.get('custom_fields')
        relations = raw.get('relations')
        return Issue(
            id=raw['id'],
            project_id=raw.get('project', {}).get('id'),
            tracker_name=raw.get('tracker', {}).get('name'),
            status_name=raw.get('status', {}).get('name'),
            priority_name=raw.get('priority', {}).get('name'),
            author_id=raw.get('author', {}).get('id'),
            author_name=raw.get('author', {}).get('name'),
            assigned_to_id=raw.get('assigned_to', {}).get('id') if raw.get('assigned_to') else None,
            assigned_to_name=raw.get('assigned_to', {}).get('name') if raw.get('assigned_to') else None,
            parent_id=parent.get('id'),
            category_id=category.get('id'),
            category_name=category.get('name'),
            fixed_version_id=fixed_version.get('id'),
            fixed_version_name=fixed_version.get('name'),
            subject=raw.get('subject'),
            description=raw.get('description'),
            start_date=raw.get('start_date'),
            due_date=raw.get('due_date'),
            done_ratio=raw.get('done_ratio'),
            estimated_hours=raw.get('estimated_hours'),
            spent_hours=raw.get('spent_hours'),
            total_spent_hours=raw.get('total_spent_hours'),
            custom_fields_json=json.dumps(custom_fields) if custom_fields else None,
            relations_json=json.dumps(relations) if relations else None,
            created_on=raw.get('created_on'),
            updated_on=raw.get('updated_on'),
            closed_on=raw.get('closed_on'),
        )

    def convert(self, issue: Issue, ctx) -> Iterable[object]:
        board_id = ctx.scope.domain_id()
        tracker = issue.tracker_name or 'Task'
        issue_id = domain_id(Issue, ctx.connection.id, issue.id)
        original_estimate_minutes = self._hours_to_minutes(issue.estimated_hours)
        time_spent_minutes = self._hours_to_minutes(issue.spent_hours)
        time_remaining_minutes = None
        if original_estimate_minutes is not None and time_spent_minutes is not None:
            time_remaining_minutes = max(0, original_estimate_minutes - time_spent_minutes)
        i = ticket.Issue(
            url=self._issue_url(ctx.connection.endpoint, issue.id),
            issue_key=str(issue.id),
            title=issue.subject or '',
            description=issue.description,
            type=self._resolve_issue_type(issue.tracker_name, ctx.scope_config),
            original_type=tracker,
            status=self._resolve_issue_status(issue.status_name, ctx.scope_config),
            original_status=issue.status_name,
            created_date=issue.created_on,
            updated_date=issue.updated_on,
            resolution_date=issue.closed_on,
            due_date=self._due_date(issue.due_date),
            creator_id=str(issue.author_id) if issue.author_id else None,
            creator_name=issue.author_name,
            assignee_id=str(issue.assigned_to_id) if issue.assigned_to_id else None,
            assignee_name=issue.assigned_to_name,
            parent_issue_id=(
                domain_id(Issue, ctx.connection.id, issue.parent_id)
                if issue.parent_id else None
            ),
            is_subtask=issue.parent_id is not None,
            priority=issue.priority_name,
            component=issue.category_name,
            original_project=ctx.scope.name,
            lead_time_minutes=self._lead_time_minutes(issue),
            original_estimate_minutes=original_estimate_minutes,
            time_spent_minutes=time_spent_minutes,
            time_remaining_minutes=time_remaining_minutes,
            fix_versions=issue.fixed_version_name,
        )
        yield i
        yield ticket.BoardIssue(board_id=board_id, issue_id=i.id)
        if issue.assigned_to_id:
            yield ticket.IssueAssignee(
                issue_id=i.id,
                assignee_id=str(issue.assigned_to_id),
                assignee_name=issue.assigned_to_name,
            )
        if issue.category_name:
            yield ticket.IssueLabel(issue_id=issue_id, label_name=issue.category_name)
        for relation in self._relations(issue.relations_json):
            source_id = relation.get('issue_id')
            target_id = relation.get('issue_to_id')
            relation_type = relation.get('relation_type')
            if not (source_id and target_id and relation_type):
                continue
            yield ticket.IssueRelationship(
                source_issue_id=domain_id(Issue, ctx.connection.id, source_id),
                target_issue_id=domain_id(Issue, ctx.connection.id, target_id),
                original_type=relation_type,
            )

    @staticmethod
    def _issue_url(endpoint, issue_id: int) -> str:
        return f'{str(endpoint).rstrip("/")}/issues/{issue_id}'

    @staticmethod
    def _hours_to_minutes(hours: Optional[float]) -> Optional[int]:
        if hours is None:
            return None
        return round(hours * 60)

    def _resolve_issue_type(self, tracker_name: Optional[str], scope_config: ScopeConfig) -> str:
        tracker = tracker_name or 'Task'
        configured = self._standard_type((scope_config.type_mappings or {}).get(tracker))
        if configured:
            return configured.upper()
        return _DEFAULT_TYPE_BY_TRACKER.get(tracker, 'TASK')

    def _standard_type(self, entry) -> Optional[str]:
        if entry is None:
            return None
        if isinstance(entry, TypeMapping):
            return entry.standard_type
        if isinstance(entry, dict):
            value = entry.get('standardType') or entry.get('standard_type')
            return str(value) if value else None
        return None

    def _resolve_issue_status(self, status_name: Optional[str], scope_config: ScopeConfig) -> str:
        if not status_name:
            return 'OTHER'
        configured = self._standard_status((scope_config.status_mappings or {}).get(status_name))
        if configured:
            return configured.upper()
        return _DEFAULT_STATUS_BY_NAME.get(status_name, 'OTHER')

    def _standard_status(self, entry) -> Optional[str]:
        if entry is None:
            return None
        if isinstance(entry, StatusMapping):
            return entry.standard_status
        if isinstance(entry, dict):
            value = entry.get('standardStatus') or entry.get('standard_status')
            return str(value) if value else None
        return None

    @staticmethod
    def _due_date(value) -> Optional[datetime.datetime]:
        if value is None:
            return None
        if isinstance(value, datetime.datetime):
            return value
        if isinstance(value, datetime.date):
            return datetime.datetime.combine(value, datetime.time.min)
        if isinstance(value, str) and value:
            return datetime.datetime.combine(
                datetime.date.fromisoformat(value), datetime.time.min
            )
        return None

    @staticmethod
    def _relations(relations_json: Optional[str]) -> list[dict]:
        if not relations_json:
            return []
        try:
            relations = json.loads(relations_json)
        except json.JSONDecodeError:
            return []
        return relations if isinstance(relations, list) else []

    @staticmethod
    def _parse_datetime(value) -> Optional[datetime.datetime]:
        if value is None:
            return None
        if isinstance(value, datetime.datetime):
            return value
        if isinstance(value, str):
            return datetime.datetime.fromisoformat(value.replace('Z', '+00:00'))
        return None

    @staticmethod
    def _lead_time_minutes(issue: Issue) -> Optional[int]:
        created = Issues._parse_datetime(issue.created_on)
        closed = Issues._parse_datetime(issue.closed_on)
        if not created or not closed:
            return None
        return int((closed - created).total_seconds() // 60)
