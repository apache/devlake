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

from redmine.api import RedmineAPI
from redmine.models import Issue, TimeEntry
import pydevlake.domain_layer.ticket as ticket
from pydevlake import Stream, DomainType, domain_id


class TimeEntries(Stream):
    tool_model = TimeEntry
    domain_types = [DomainType.TICKET]
    domain_models = [ticket.IssueWorklog]

    def collect(self, state, context) -> Iterable[tuple[object, dict]]:
        if not getattr(context.scope_config, 'collect_time_entries', False):
            return
        with RedmineAPI(context.connection) as api:
            for raw in api.time_entries(context.scope.id):
                raw['x_request_url'] = f'{api.base_url}/time_entries.json'
                yield raw, state

    def extract(self, raw: dict) -> TimeEntry:
        return TimeEntry(
            id=raw['id'],
            project_id=raw.get('project', {}).get('id') if raw.get('project') else None,
            issue_id=raw.get('issue', {}).get('id') if raw.get('issue') else None,
            user_id=raw.get('user', {}).get('id') if raw.get('user') else None,
            user_name=raw.get('user', {}).get('name') if raw.get('user') else None,
            hours=raw.get('hours'),
            comments=raw.get('comments'),
            spent_on=raw.get('spent_on'),
            created_on=raw.get('created_on'),
            updated_on=raw.get('updated_on'),
        )

    def convert(self, te: TimeEntry, ctx) -> Iterable[object]:
        if not te.issue_id:
            return
        issue_id = domain_id(Issue, ctx.connection.id, te.issue_id)
        yield ticket.IssueWorklog(
            issue_id=issue_id,
            author_id=str(te.user_id) if te.user_id else None,
            comment=te.comments,
            time_spent_minutes=round(te.hours * 60) if te.hours else None,
            started_date=te.spent_on,
            logged_date=te.created_on,
        )
