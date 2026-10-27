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

from redmine.api import RedmineAPI
from redmine.models import Issue, Journal, Project
from redmine.streams.issues import Issues
import pydevlake.domain_layer.ticket as ticket
from pydevlake import Stream, DomainType, domain_id
from pydevlake.collection_time import collection_time_after, format_collection_date


_ATTR_FIELD_MAP = {
    'assigned_to_id': 'assignee',
    'status_id': 'status',
    'priority_id': 'priority',
    'category_id': 'category',
    'fixed_version_id': 'fix_version',
    'tracker_id': 'type',
    'parent_issue_id': 'parent',
    'done_ratio': 'progress',
    'estimated_hours': 'original_estimate',
    'start_date': 'start_date',
    'due_date': 'due_date',
    'subject': 'title',
    'description': 'description',
    'is_private': 'is_private',
}


class IssueJournals(Stream):
    tool_model = Journal
    domain_types = [DomainType.TICKET]
    domain_models = [ticket.IssueComment, ticket.IssueChangelogs]

    def collect(self, state, context) -> Iterable[tuple[object, dict]]:
        if not getattr(context.scope_config, 'collect_journals', False):
            return
        project: Project = context.scope
        run_state = dict(state)
        use_incremental = context.options.get('incremental')
        updated_since = run_state.get('updated_since') if use_incremental else None
        created_on = self._created_on_filter(context)
        with RedmineAPI(context.connection) as api:
            for raw_issue in api.issues(
                project.id,
                created_on=created_on,
                updated_since=updated_since,
            ):
                Issues._track_updated_since(run_state, raw_issue.get('updated_on'))
                issue_id = raw_issue['id']
                full_issue = api.issue(issue_id, include_journals=True)
                for journal in full_issue.get('journals', []):
                    journal['_issue_id'] = issue_id
                    journal['x_request_url'] = f'{api.base_url}/issues/{issue_id}.json'
                    yield journal, run_state

    @staticmethod
    def _created_on_filter(context) -> Optional[str]:
        if context.options.get('incremental'):
            return None
        time_after = collection_time_after(context)
        if not time_after:
            return None
        return f'>={format_collection_date(time_after)}'

    def extract(self, raw: dict) -> Journal:
        details = raw.get('details')
        return Journal(
            id=raw['id'],
            issue_id=raw.get('_issue_id'),
            user_id=raw.get('user', {}).get('id') if raw.get('user') else None,
            user_name=raw.get('user', {}).get('name') if raw.get('user') else None,
            notes=raw.get('notes'),
            details_json=json.dumps(details) if details else None,
            created_on=raw.get('created_on'),
            private_notes=raw.get('private_notes', False),
        )

    def convert(self, j: Journal, ctx) -> Iterable[object]:
        if not j.issue_id:
            return
        issue_id = domain_id(Issue, ctx.connection.id, j.issue_id)
        if j.notes:
            yield ticket.IssueComment(
                issue_id=issue_id,
                body=j.notes,
                account_id=str(j.user_id) if j.user_id else None,
                created_date=j.created_on,
            )
        for idx, detail in enumerate(self._details(j.details_json)):
            changelog = self._detail_to_changelog(j, issue_id, detail, idx, ctx)
            if changelog is not None:
                yield changelog

    @staticmethod
    def _details(details_json: Optional[str]) -> list[dict]:
        if not details_json:
            return []
        try:
            details = json.loads(details_json)
        except json.JSONDecodeError:
            return []
        return details if isinstance(details, list) else []

    def _detail_to_changelog(
        self,
        journal: Journal,
        issue_id: str,
        detail: dict,
        idx: int,
        ctx,
    ) -> Optional[ticket.IssueChangelogs]:
        field_name = self._changelog_field_name(detail.get('property'), detail.get('name'))
        if not field_name:
            return None
        old_value = detail.get('old_value')
        new_value = detail.get('new_value')
        if old_value in (None, '') and new_value in (None, ''):
            return None
        return ticket.IssueChangelogs(
            id=domain_id(Journal, ctx.connection.id, journal.id, idx),
            issue_id=issue_id,
            author_id=str(journal.user_id) if journal.user_id else None,
            author_name=journal.user_name,
            field_id=detail.get('name'),
            field_name=field_name,
            original_from_value=old_value,
            original_to_value=new_value,
            from_value=old_value,
            to_value=new_value,
            created_date=journal.created_on,
        )

    @staticmethod
    def _changelog_field_name(property_name: Optional[str], name: Optional[str]) -> Optional[str]:
        if not name:
            return None
        if property_name == 'attr':
            return _ATTR_FIELD_MAP.get(name, name)
        if property_name == 'cf':
            return f'custom_field:{name}'
        if property_name == 'relation':
            return 'relation'
        return name
