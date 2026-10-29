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

from typing import Iterable, Optional, Union

from glpi.api import GlpiAPI
from glpi.models import (
    Followup,
    GLPI_TICKET_LINK_TYPES,
    GLPI_TICKET_STATUS,
    GLPI_TICKET_STATUS_TO_DEVLAKE,
    GLPI_PRIORITY,
    GLPI_TICKET_TYPE,
    Group,
    ITILSolution,
    Ticket,
    TicketGroup,
    TicketLink,
    TicketLog,
    TicketTask,
    TicketUser,
)
from glpi.parsing import parse_glpi_dropdown, parse_glpi_user, user_account_id
from glpi.streams.tickets import Tickets
from glpi.timezone import connection_timezone, to_utc
from pydevlake import DomainType, Substream, domain_id
from pydevlake.context import Context
from pydevlake.extractor import autoextract
from pydevlake.subtasks import Collector, SubstreamCollector
from sqlmodel import Session
import pydevlake.domain_layer.ticket as ticket


# GLPI REST bug: GET /Ticket/{id}/Ticket_Ticket ignores the parent ticket id and
# returns every link in the instance (other sub-item endpoints filter correctly).
# Workaround: paginate Ticket_Ticket once, then filter to collected ticket ids.
# https://forum.glpi-project.org/viewtopic.php?id=288309
class TicketLinksCollector(Collector):

    def fetch(self, state, session: Session, ctx: Context):
        if not ctx.scope_config.collect_ticket_links:
            return
        ticket_ids = self._collected_ticket_ids(session, self._params(ctx))
        if not ticket_ids:
            return
        with GlpiAPI(ctx.connection) as api:
            for raw in api.iter_ticket_links():
                if not self._link_involves_tickets(raw, ticket_ids):
                    continue
                raw['x_request_url'] = f'{api.base_url}/apirest.php/Ticket_Ticket'
                raw['x_request_input'] = {}
                yield raw, state

    @staticmethod
    def _collected_ticket_ids(session: Session, params: str) -> set[int]:
        return {
            ticket.id
            for ticket in session.query(Ticket).where(
                Ticket.raw_data_params == params,
            ).all()
        }

    @staticmethod
    def _link_involves_tickets(raw: dict, ticket_ids: set[int]) -> bool:
        linked = (
            raw.get('tickets_id_1') or raw.get('tickets_id1'),
            raw.get('tickets_id_2') or raw.get('tickets_id2'),
        )
        return any(int(ticket_id) in ticket_ids for ticket_id in linked if ticket_id is not None)


class GlpiSubstreamCollector(SubstreamCollector):

    def fetch(self, state, session, ctx: Context):
        parent_model = self.stream.parent_stream.tool_model
        query = session.query(parent_model).where(
            parent_model.raw_data_params == self._params(ctx),
        )
        parents = query.all()
        if not parents:
            return
        with GlpiAPI(ctx.connection) as api:
            for parent in parents:
                yield from self.stream.collect(state, ctx, parent, api)


class GlpiTicketSubstream(Substream):
    parent_stream = Tickets
    glpi_resource: str
    optional_collect_flag: Optional[str] = None
    optional_collect_default: bool = False

    def __init__(self, plugin_name: str):
        super().__init__(plugin_name)
        self.collector = GlpiSubstreamCollector(self)

    def collect(
        self,
        state,
        context,
        parent: Ticket,
        api: GlpiAPI,
    ) -> Iterable[tuple[object, dict]]:
        if self.optional_collect_flag is not None and not getattr(
            context.scope_config,
            self.optional_collect_flag,
            self.optional_collect_default,
        ):
            return
        ticket_id = parent.id
        for raw in api.ticket_subitems(ticket_id, self.glpi_resource):
            raw['tickets_id'] = ticket_id
            if self.glpi_resource == 'ITILFollowup':
                raw['items_id'] = ticket_id
            raw['x_request_url'] = f'{api.base_url}/apirest.php/Ticket/{ticket_id}/{self.glpi_resource}'
            raw['x_request_input'] = {'TicketId': ticket_id}
            yield raw, state


class TicketUsers(GlpiTicketSubstream):
    tool_model = TicketUser
    domain_types = [DomainType.TICKET]
    domain_models = [ticket.IssueAssignee]
    glpi_resource = 'Ticket_User'
    optional_collect_flag = 'collect_assignees'
    optional_collect_default = True

    def extract(self, raw: dict) -> TicketUser:
        model = autoextract(raw, self.tool_model)
        name, user_id = parse_glpi_dropdown(raw.get('users_id'))
        model.users_id = user_id
        model.user_name = name
        return model

    def convert(self, tu: TicketUser, ctx: Context) -> Iterable[ticket.IssueAssignee]:
        result = self._convert_ticket_user_assignee(tu, ctx)
        if result is not None:
            yield result

    def _convert_ticket_user_assignee(self, tu: TicketUser, ctx: Context) -> Optional[ticket.IssueAssignee]:
        assigned_user_type = 2
        if tu.type != assigned_user_type or tu.tickets_id is None:
            return None
        user_id = tu.users_id
        if user_id is None:
            return None
        issue_id = domain_id(Ticket, ctx.connection.id, tu.tickets_id)
        return ticket.IssueAssignee(
            issue_id=issue_id,
            assignee_id=user_account_id(ctx.connection.id, user_id),
            assignee_name=tu.user_name,
        )


def _group_account_id(connection_id: int, group_id: Optional[Union[int, str]]) -> Optional[str]:
    return domain_id(Group, connection_id, int(group_id)) if group_id is not None else None


class TicketGroups(GlpiTicketSubstream):
    tool_model = TicketGroup
    domain_types = [DomainType.TICKET]
    domain_models = [ticket.IssueAssignee]
    glpi_resource = 'Group_Ticket'
    optional_collect_flag = 'collect_assignees'
    optional_collect_default = True

    def extract(self, raw: dict) -> TicketGroup:
        model = autoextract(raw, self.tool_model)
        name, group_id = parse_glpi_dropdown(raw.get('groups_id'))
        model.groups_id = group_id
        model.group_name = name
        return model

    def convert(self, tg: TicketGroup, ctx: Context) -> Iterable[ticket.IssueAssignee]:
        result = self._convert_ticket_group_assignee(tg, ctx)
        if result is not None:
            yield result

    def _convert_ticket_group_assignee(self, tg: TicketGroup, ctx: Context) -> Optional[ticket.IssueAssignee]:
        assigned_group_type = 2
        if tg.type != assigned_group_type or tg.tickets_id is None or tg.groups_id is None:
            return None
        issue_id = domain_id(Ticket, ctx.connection.id, tg.tickets_id)
        return ticket.IssueAssignee(
            issue_id=issue_id,
            assignee_id=_group_account_id(ctx.connection.id, tg.groups_id),
            assignee_name=tg.group_name,
        )


_GLPI_TICKET_STATUS_NAME_TO_DEVLAKE = {
    GLPI_TICKET_STATUS[code]: status
    for code, status in GLPI_TICKET_STATUS_TO_DEVLAKE.items()
}


class TicketFollowups(GlpiTicketSubstream):
    tool_model = Followup
    domain_types = [DomainType.TICKET]
    domain_models = [ticket.IssueComment]
    glpi_resource = 'ITILFollowup'
    optional_collect_flag = 'collect_followups'

    def convert(self, followup: Followup, ctx: Context) -> Iterable[ticket.IssueComment]:
        result = self._convert_followup_comment(followup, ctx)
        if result is not None:
            yield result

    def _convert_followup_comment(self, followup: Followup, ctx: Context) -> Optional[ticket.IssueComment]:
        if followup.is_private in (1, '1', True) or followup.items_id is None:
            return None
        tz = connection_timezone(ctx.connection.timezone)
        issue_id = domain_id(Ticket, ctx.connection.id, followup.items_id)
        comment = ticket.IssueComment(
            issue_id=issue_id,
            body=followup.content,
            account_id=user_account_id(ctx.connection.id, followup.users_id),
            created_date=to_utc(followup.date, tz),
            updated_date=to_utc(followup.date_mod, tz),
        )
        if followup.id is not None:
            comment.id = domain_id(Followup, ctx.connection.id, followup.id)
        return comment


class TicketTasks(GlpiTicketSubstream):
    tool_model = TicketTask
    domain_types = [DomainType.TICKET]
    domain_models = [ticket.IssueWorklog]
    glpi_resource = 'TicketTask'
    optional_collect_flag = 'collect_tasks'

    def convert(self, task: TicketTask, ctx: Context) -> Iterable[ticket.IssueWorklog]:
        result = self._convert_ticket_task_worklog(task, ctx)
        if result is not None:
            yield result

    def _convert_ticket_task_worklog(self, task: TicketTask, ctx: Context) -> Optional[ticket.IssueWorklog]:
        if not task.actiontime or task.tickets_id is None:
            return None
        tz = connection_timezone(ctx.connection.timezone)
        issue_id = domain_id(Ticket, ctx.connection.id, task.tickets_id)
        worklog = ticket.IssueWorklog(
            issue_id=issue_id,
            author_id=user_account_id(ctx.connection.id, task.users_id),
            comment=task.content,
            time_spent_minutes=int(task.actiontime) // 60,
            started_date=to_utc(task.begin, tz),
            logged_date=to_utc(task.begin, tz),
        )
        if task.id is not None:
            worklog.id = domain_id(TicketTask, ctx.connection.id, task.id)
        return worklog


class TicketLogs(GlpiTicketSubstream):
    tool_model = TicketLog
    domain_types = [DomainType.TICKET]
    domain_models = [ticket.IssueChangelogs]
    glpi_resource = 'Log'
    optional_collect_flag = 'collect_logs'

    def convert(self, log: TicketLog, ctx: Context) -> Iterable[ticket.IssueChangelogs]:
        result = self._convert_ticket_log_changelog(log, ctx)
        if result is not None:
            yield result

    def _convert_ticket_log_changelog(self, log: TicketLog, ctx: Context) -> Optional[ticket.IssueChangelogs]:
        if log.tickets_id is None:
            return None
        resolved = self._devlake_changelog_from_glpi_ticket_log(log)
        if not resolved:
            return None
        tz = connection_timezone(ctx.connection.timezone)
        issue_id = domain_id(Ticket, ctx.connection.id, log.tickets_id)
        author_name, author_id = parse_glpi_user(log.user_name)
        itemtype_link = resolved.get('itemtype_link')
        from_value = resolved['from_value']
        to_value = resolved['to_value']
        if resolved['field_name'] in ('assignee', 'requester', 'observer'):
            from_value = self._account_id_from_glpi_ref(
                ctx.connection.id, resolved['original_from_value'], itemtype_link
            )
            to_value = self._account_id_from_glpi_ref(
                ctx.connection.id, resolved['original_to_value'], itemtype_link
            )
        changelog = ticket.IssueChangelogs(
            issue_id=issue_id,
            author_id=user_account_id(ctx.connection.id, author_id),
            author_name=author_name,
            field_id=resolved['field_id'],
            field_name=resolved['field_name'],
            original_from_value=resolved['original_from_value'],
            original_to_value=resolved['original_to_value'],
            from_value=from_value,
            to_value=to_value,
            created_date=to_utc(log.date_mod, tz),
        )
        if log.id is not None:
            changelog.id = domain_id(TicketLog, ctx.connection.id, log.id)
        return changelog

    def _devlake_changelog_from_glpi_ticket_log(self, log: TicketLog) -> Optional[dict]:
        field_name = self._changelog_field_name(log)
        if not field_name:
            return None

        old_value = self._clean_changelog_value(log.old_value)
        new_value = self._clean_changelog_value(log.new_value)
        if not old_value and not new_value:
            return None

        result = {
            'field_name': field_name,
            'field_id': str(log.id_search_option) if log.id_search_option is not None else None,
            'original_from_value': old_value,
            'original_to_value': new_value,
            'from_value': old_value,
            'to_value': new_value,
        }

        if field_name == 'status':
            result['from_value'] = self._devlake_status_from_glpi_name(old_value)
            result['to_value'] = self._devlake_status_from_glpi_name(new_value)
        elif field_name in ('priority', 'urgency', 'impact'):
            result['from_value'] = self._devlake_priority_from_glpi_name(old_value)
            result['to_value'] = self._devlake_priority_from_glpi_name(new_value)
        elif field_name == 'type':
            result['from_value'] = self._devlake_type_from_glpi_name(old_value)
            result['to_value'] = self._devlake_type_from_glpi_name(new_value)
        elif field_name in ('assignee', 'requester', 'observer'):
            _, from_id = parse_glpi_user(old_value)
            _, to_id = parse_glpi_user(new_value)
            result['from_value'] = from_id
            result['to_value'] = to_id
            result['original_from_value'] = from_id or old_value
            result['original_to_value'] = to_id or new_value
            result['itemtype_link'] = log.itemtype_link or 'User'

        return result

    def _changelog_field_name(self, log: TicketLog) -> Optional[str]:
        history_add_relation = 15
        history_del_relation = 16
        if log.linked_action in (history_add_relation, history_del_relation):
            if log.itemtype_link == 'Group':
                return 'assignee'
            return {
                4: 'requester',
                5: 'assignee',
                66: 'observer',
            }.get(log.id_search_option)
        if log.linked_action and log.linked_action != 0:
            return None
        return {
            3: 'priority',
            4: 'requester',
            5: 'assignee',
            7: 'category',
            10: 'urgency',
            11: 'impact',
            12: 'status',
            14: 'type',
            66: 'observer',
        }.get(log.id_search_option)

    def _clean_changelog_value(self, value: Optional[str]) -> Optional[str]:
        if value is None:
            return None
        value = value.strip()
        if not value or value == '\xa0':
            return None
        return value

    def _devlake_status_from_glpi_name(self, value: Optional[str]) -> Optional[str]:
        if not value:
            return None
        text = value.strip()
        if text.isdigit():
            return GLPI_TICKET_STATUS_TO_DEVLAKE.get(int(text))
        return _GLPI_TICKET_STATUS_NAME_TO_DEVLAKE.get(text)

    def _devlake_priority_from_glpi_name(self, value: Optional[str]) -> Optional[str]:
        if not value:
            return None
        text = value.strip()
        if text.isdigit():
            return GLPI_PRIORITY.get(int(text))
        return text if text in GLPI_PRIORITY.values() else text

    def _devlake_type_from_glpi_name(self, value: Optional[str]) -> Optional[str]:
        if not value:
            return None
        text = value.strip()
        glpi_type_name_to_devlake = {
            "Incident": 'INCIDENT',
            "Request": 'TASK',
        }
        if text.isdigit():
            name = GLPI_TICKET_TYPE.get(int(text))
            return glpi_type_name_to_devlake.get(name) if name else None
        return glpi_type_name_to_devlake.get(text)


    def _account_id_from_glpi_ref(
        self,
        connection_id: int,
        value: Optional[str],
        itemtype_link: Optional[str] = None,
    ) -> Optional[str]:
        if not value:
            return None
        if itemtype_link == 'Group':
            _, group_id = parse_glpi_dropdown(value)
            return _group_account_id(connection_id, group_id)
        _, user_id = parse_glpi_user(value)
        if user_id is None and str(value).strip().isdigit():
            user_id = str(value).strip()
        return user_account_id(connection_id, user_id)


class TicketLinks(Substream):
    # Uses TicketLinksCollector; see comment there for why this is not a per-ticket substream.
    parent_stream = Tickets
    tool_model = TicketLink
    domain_types = [DomainType.TICKET]
    domain_models = [ticket.IssueRelationship]

    def __init__(self, plugin_name: str):
        super().__init__(plugin_name)
        self.collector = TicketLinksCollector(self)

    def convert(self, link: TicketLink, ctx: Context) -> Iterable[ticket.IssueRelationship]:
        result = self._convert_ticket_link(link, ctx)
        if result is not None:
            yield result

    def _convert_ticket_link(self, link: TicketLink, ctx: Context) -> Optional[ticket.IssueRelationship]:
        t1, t2 = link.tickets_id_1, link.tickets_id_2
        if not (t1 and t2):
            return None
        link_type = GLPI_TICKET_LINK_TYPES.get(link.link, str(link.link or 'linked'))
        return ticket.IssueRelationship(
            source_issue_id=domain_id(Ticket, ctx.connection.id, t1),
            target_issue_id=domain_id(Ticket, ctx.connection.id, t2),
            original_type=link_type,
        )


class TicketSolutions(GlpiTicketSubstream):
    tool_model = ITILSolution
    domain_types = [DomainType.TICKET]
    domain_models = []
    glpi_resource = 'ITILSolution'
    optional_collect_flag = 'collect_solutions'

    @property
    def subtasks(self):
        return [self.collector, self.extractor]
