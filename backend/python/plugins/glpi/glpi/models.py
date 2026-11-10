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
from typing import Optional

from pydantic import AnyUrl, SecretStr, field_validator
from sqlalchemy import BigInteger, Column

from pydevlake import Field, ScopeConfig as BaseScopeConfig
from pydevlake.model import Connection as BaseConnection, ToolModel, ToolScope

from glpi.migrations import *
from glpi.timezone import validate_timezone_name


class Connection(BaseConnection):
    endpoint: AnyUrl
    token: SecretStr
    secret_key: SecretStr
    timezone: str

    @field_validator('timezone')
    @classmethod
    def validate_timezone(cls, timezone: str) -> str:
        return validate_timezone_name(timezone)


class ScopeConfig(BaseScopeConfig):
    collect_assignees: bool = True
    collect_followups: bool = False
    collect_ticket_links: bool = False
    collect_logs: bool = False
    collect_tasks: bool = False
    collect_solutions: bool = False


class Entity(ToolScope, table=True):
    completename: Optional[str] = None


GLPI_TICKET_STATUS = {
    1: "New",
    2: "Processing (assigned)",
    3: "Processing (planned)",
    4: "Pending",
    5: "Solved",
    6: "Closed",
}

GLPI_TICKET_STATUS_TO_DEVLAKE = {
    1: 'TODO',
    2: 'IN_PROGRESS',
    3: 'IN_PROGRESS',
    4: 'IN_PROGRESS',
    5: 'DONE',
    6: 'DONE',
}

GLPI_TICKET_TYPE = {
    1: "Incident",
    2: "Request",
}

GLPI_PRIORITY = {
    1: "Very Low",
    2: "Low",
    3: "Medium",
    4: "High",
    5: "Very High",
    6: "Major",
}

GLPI_TICKET_LINK_TYPES = {
    1: 'linked',
    2: 'duplicate',
    3: 'parent',
    4: 'child',
}


class User(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str] = None
    realname: Optional[str] = None
    firstname: Optional[str] = None
    email: Optional[str] = None


class Ticket(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str] = None
    content: Optional[str] = None
    date: Optional[datetime.datetime] = None
    closedate: Optional[datetime.datetime] = None
    solvedate: Optional[datetime.datetime] = None
    date_mod: Optional[datetime.datetime] = None
    date_creation: Optional[datetime.datetime] = None
    status: Optional[int] = None
    type: Optional[int] = None
    urgency: Optional[int] = None
    impact: Optional[int] = None
    priority: Optional[int] = None
    entities_id: Optional[str] = None
    itilcategories_id: Optional[int] = None
    category_name: Optional[str] = None
    locations_id: Optional[int] = None
    location_name: Optional[str] = None
    suppliers_id: Optional[int] = None
    supplier_name: Optional[str] = None
    requesttypes_id: Optional[int] = None
    requesttype_name: Optional[str] = None
    users_id_recipient: Optional[int] = None
    users_id_lastupdater: Optional[int] = None
    time_to_resolve: Optional[datetime.datetime] = None
    time_to_own: Optional[datetime.datetime] = None
    internal_time_to_resolve: Optional[datetime.datetime] = None
    internal_time_to_own: Optional[datetime.datetime] = None
    actiontime: Optional[int] = None
    solve_delay_stat: Optional[int] = None
    takeintoaccount_delay_stat: Optional[int] = None
    requester_name: Optional[str] = None
    assignee_id: Optional[int] = None
    assignee_name: Optional[str] = None
    parent_ticket_id: Optional[int] = None
    solution_content: Optional[str] = None
    plugin_fields_json: Optional[str] = None


class ITILCategory(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str] = None
    completename: Optional[str] = None


class Location(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str] = None
    completename: Optional[str] = None


class Supplier(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str] = None


class RequestType(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str] = None


class TicketLabelRecord(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    label_name: Optional[str] = None


class TicketCustomField(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int] = None
    field_id: Optional[str] = None
    field_value: Optional[str] = None


class TicketUser(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int] = None
    users_id: Optional[int] = None
    user_name: Optional[str] = None
    type: Optional[int] = None


class TicketGroup(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int] = None
    groups_id: Optional[int] = None
    group_name: Optional[str] = None
    type: Optional[int] = None


class Followup(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    items_id: Optional[int] = None
    users_id: Optional[int] = None
    content: Optional[str] = None
    date: Optional[datetime.datetime] = None
    date_mod: Optional[datetime.datetime] = None
    is_private: Optional[int] = None


class TicketTask(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int] = None
    users_id: Optional[int] = None
    content: Optional[str] = None
    begin: Optional[datetime.datetime] = None
    end: Optional[datetime.datetime] = None
    actiontime: Optional[int] = None


class TicketLog(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int] = None
    items_id: Optional[int] = None
    itemtype: Optional[str] = None
    itemtype_link: Optional[str] = None
    linked_action: Optional[int] = None
    user_name: Optional[str] = None
    date_mod: Optional[datetime.datetime] = None
    id_search_option: Optional[int] = None
    old_value: Optional[str] = None
    new_value: Optional[str] = None


class Group(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str] = None
    completename: Optional[str] = None


class TicketLink(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int] = None
    tickets_id_1: Optional[int] = Field(default=None, sa_column=Column('tickets_id1', BigInteger))
    tickets_id_2: Optional[int] = Field(default=None, sa_column=Column('tickets_id2', BigInteger))
    link: Optional[int] = None


class ITILSolution(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int] = None
    content: Optional[str] = None
