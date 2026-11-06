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

from typing import Generator, Iterable, Optional

from glpi.models import (
    ITILSolution,
    Ticket,
    TicketLink,
    TicketUser,
    GLPI_TICKET_STATUS,
    GLPI_TICKET_STATUS_TO_DEVLAKE,
    GLPI_TICKET_TYPE,
    GLPI_PRIORITY,
)
from glpi.parsing import user_account_id
from glpi.timezone import connection_timezone, to_utc
from sqlalchemy import or_
import pydevlake.domain_layer.ticket as ticket
from pydevlake import Stream, DomainType, domain_id
from pydevlake.context import Context
from pydevlake.model import ToolModel
from pydevlake.subtasks import Convertor
from sqlmodel import Session


class TicketDomainConvertor(Convertor):

    def process(self, tool_model: ToolModel, session: Session, ctx: Context):
        self._enrich_ticket_from_tool_records(session, ctx, tool_model)
        res = self.stream.convert(tool_model, ctx)
        if isinstance(res, Generator):
            for each in res:
                self._save(tool_model, each, session, ctx.connection.id)
        else:
            self._save(tool_model, res, session, ctx.connection.id)

    def _enrich_ticket_from_tool_records(self, session: Session, ctx: Context, ticket_model: Ticket) -> None:
        assigned_user_type = 2
        assignees = session.query(TicketUser).where(
            TicketUser.connection_id == ctx.connection.id,
            TicketUser.tickets_id == ticket_model.id,
            TicketUser.type == assigned_user_type,
        ).all()
        if assignees:
            row = assignees[0]
            ticket_model.assignee_id = row.users_id
            ticket_model.assignee_name = row.user_name

        links = session.query(TicketLink).where(
            TicketLink.connection_id == ctx.connection.id,
            or_(
                TicketLink.tickets_id_1 == ticket_model.id,
                TicketLink.tickets_id_2 == ticket_model.id,
            ),
        ).all()
        link_dicts = [
            {'link': link.link, 'tickets_id_1': link.tickets_id_1, 'tickets_id_2': link.tickets_id_2}
            for link in links
        ]
        parent_id = self._parent_ticket_id_from_links(link_dicts, ticket_model.id)
        if parent_id is not None:
            ticket_model.parent_ticket_id = parent_id

        solutions = session.query(ITILSolution).where(
            ITILSolution.connection_id == ctx.connection.id,
            ITILSolution.tickets_id == ticket_model.id,
        ).order_by(ITILSolution.id).all()
        if solutions:
            ticket_model.solution_content = solutions[-1].content

    def _parent_ticket_id_from_links(self, links: list, ticket_id: int) -> Optional[int]:
        glpi_link_son_of = 3
        glpi_link_parent_of = 4
        for link in links:
            link_type = link.get('link')
            t1, t2 = link.get('tickets_id_1'), link.get('tickets_id_2')
            if link_type == glpi_link_son_of and t1 == ticket_id:
                return t2
            if link_type == glpi_link_parent_of and t2 == ticket_id:
                return t1
        return None


class TicketDomain(Stream):
    tool_model = Ticket
    domain_types = [DomainType.TICKET]
    domain_models = [
        ticket.Issue,
        ticket.BoardIssue,
        ticket.IssueLabel,
    ]

    def __init__(self, plugin_name: str):
        super().__init__(plugin_name)
        self.convertor = TicketDomainConvertor(self)

    @property
    def subtasks(self):
        return [self.convertor]

    def convert(self, t: Ticket, ctx: Context) -> Iterable[object]:
        board_id = ctx.scope.domain_id()
        tz = connection_timezone(ctx.connection.timezone)
        glpi_type_to_devlake = {
            1: 'INCIDENT',
            2: 'TASK',
        }
        issue_id = domain_id(Ticket, ctx.connection.id, t.id)
        issue = ticket.Issue(
            url=self._ticket_url(ctx.connection.endpoint, t.id),
            issue_key=str(t.id),
            title=t.name or '',
            description=self._issue_description(t.content, t.solution_content),
            type=glpi_type_to_devlake.get(t.type, 'TASK'),
            original_type=GLPI_TICKET_TYPE.get(t.type) or (str(t.type) if t.type else None),
            status=GLPI_TICKET_STATUS_TO_DEVLAKE.get(t.status, 'OTHER'),
            original_status=GLPI_TICKET_STATUS.get(t.status),
            created_date=to_utc(t.date or t.date_creation, tz),
            updated_date=to_utc(t.date_mod, tz),
            resolution_date=to_utc(t.solvedate or t.closedate, tz),
            due_date=to_utc(t.time_to_resolve, tz),
            creator_id=user_account_id(ctx.connection.id, t.users_id_recipient),
            creator_name=t.requester_name,
            assignee_id=user_account_id(ctx.connection.id, t.assignee_id),
            assignee_name=t.assignee_name,
            parent_issue_id=(
                domain_id(Ticket, ctx.connection.id, t.parent_ticket_id)
                if t.parent_ticket_id else None
            ),
            is_subtask=t.parent_ticket_id is not None,
            priority=GLPI_PRIORITY.get(t.priority),
            urgency=GLPI_PRIORITY.get(t.urgency),
            severity=GLPI_PRIORITY.get(t.impact),
            component=t.category_name,
            original_project=t.entities_id,
            lead_time_minutes=t.solve_delay_stat // 60 if t.solve_delay_stat else None,
        )
        yield issue
        yield ticket.BoardIssue(board_id=board_id, issue_id=issue.id)
        if t.category_name:
            yield ticket.IssueLabel(issue_id=issue_id, label_name=t.category_name)

    def _issue_description(self, content: Optional[str], solution: Optional[str]) -> Optional[str]:
        if not solution:
            return content
        if not content:
            return solution
        return f'{content}\n\n--- Solution ---\n{solution}'

    @staticmethod
    def _ticket_url(endpoint: str, ticket_id: int) -> str:
        base = str(endpoint).rstrip('/')
        return f'{base}/front/ticket.form.php?id={ticket_id}'
