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

from pydantic import AnyUrl, SecretStr, validator
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

    @validator('timezone')
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
    completename: Optional[str]


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
    name: Optional[str]
    realname: Optional[str]
    firstname: Optional[str]
    email: Optional[str]


class Ticket(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str]
    content: Optional[str]
    date: Optional[datetime.datetime]
    closedate: Optional[datetime.datetime]
    solvedate: Optional[datetime.datetime]
    date_mod: Optional[datetime.datetime]
    date_creation: Optional[datetime.datetime]
    status: Optional[int]
    type: Optional[int]
    urgency: Optional[int]
    impact: Optional[int]
    priority: Optional[int]
    entities_id: Optional[str]
    itilcategories_id: Optional[int]
    category_name: Optional[str]
    locations_id: Optional[int]
    location_name: Optional[str]
    suppliers_id: Optional[int]
    supplier_name: Optional[str]
    requesttypes_id: Optional[int]
    requesttype_name: Optional[str]
    users_id_recipient: Optional[int]
    users_id_lastupdater: Optional[int]
    time_to_resolve: Optional[datetime.datetime]
    time_to_own: Optional[datetime.datetime]
    internal_time_to_resolve: Optional[datetime.datetime]
    internal_time_to_own: Optional[datetime.datetime]
    actiontime: Optional[int]
    solve_delay_stat: Optional[int]
    takeintoaccount_delay_stat: Optional[int]
    requester_name: Optional[str]
    assignee_id: Optional[int]
    assignee_name: Optional[str]
    parent_ticket_id: Optional[int]
    solution_content: Optional[str]
    plugin_fields_json: Optional[str]


class ITILCategory(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str]
    completename: Optional[str]


class Location(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str]
    completename: Optional[str]


class Supplier(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str]


class RequestType(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str]


class TicketLabelRecord(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    label_name: Optional[str]


class TicketCustomField(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int]
    field_id: Optional[str]
    field_value: Optional[str]


class TicketUser(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int]
    users_id: Optional[int]
    user_name: Optional[str]
    type: Optional[int]


class TicketGroup(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int]
    groups_id: Optional[int]
    group_name: Optional[str]
    type: Optional[int]


class Followup(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    items_id: Optional[int]
    users_id: Optional[int]
    content: Optional[str]
    date: Optional[datetime.datetime]
    date_mod: Optional[datetime.datetime]
    is_private: Optional[int]


class TicketTask(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int]
    users_id: Optional[int]
    content: Optional[str]
    begin: Optional[datetime.datetime]
    end: Optional[datetime.datetime]
    actiontime: Optional[int]


class TicketLog(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int]
    items_id: Optional[int]
    itemtype: Optional[str]
    itemtype_link: Optional[str]
    linked_action: Optional[int]
    user_name: Optional[str]
    date_mod: Optional[datetime.datetime]
    id_search_option: Optional[int]
    old_value: Optional[str]
    new_value: Optional[str]


class Group(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    name: Optional[str]
    completename: Optional[str]


class TicketLink(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int]
    tickets_id_1: Optional[int] = Field(default=None, sa_column=Column('tickets_id1', BigInteger))
    tickets_id_2: Optional[int] = Field(default=None, sa_column=Column('tickets_id2', BigInteger))
    link: Optional[int]


class ITILSolution(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    tickets_id: Optional[int]
    content: Optional[str]
