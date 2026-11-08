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

from pydantic import AnyUrl, SecretStr

from pydevlake import Field
from pydevlake.migration import migration, MigrationScriptBuilder
from pydevlake.model import Connection as BaseConnection, Model, ToolModel, ToolScope, ToolTable


@migration(20240604000001, name="initialize schemas for GLPI")
def init_schemas(b: MigrationScriptBuilder):
    class Connection(BaseConnection):
        endpoint: AnyUrl
        token: SecretStr
        secret_key: SecretStr
        timezone: str

    class Entity(ToolScope):
        completename: Optional[str]

    class ScopeConfig(ToolTable, Model):
        name: str = Field(default="default")
        entities: Optional[str]
        connection_id: Optional[int]
        collect_assignees: bool = True
        collect_followups: bool = False
        collect_ticket_links: bool = False
        collect_logs: bool = False
        collect_tasks: bool = False
        collect_solutions: bool = False
        tickets_created_after: Optional[datetime.datetime] = None

    class Ticket(ToolModel):
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

    class TicketUser(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        tickets_id: Optional[int]
        users_id: Optional[int]
        user_name: Optional[str]
        type: Optional[int]

    class TicketGroup(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        tickets_id: Optional[int]
        groups_id: Optional[int]
        group_name: Optional[str]
        type: Optional[int]

    class Followup(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        items_id: Optional[int]
        users_id: Optional[int]
        content: Optional[str]
        date: Optional[datetime.datetime]
        date_mod: Optional[datetime.datetime]
        is_private: Optional[int]

    class TicketTask(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        tickets_id: Optional[int]
        users_id: Optional[int]
        content: Optional[str]
        begin: Optional[datetime.datetime]
        end: Optional[datetime.datetime]
        actiontime: Optional[int]

    class TicketLog(ToolModel):
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

    class User(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        name: Optional[str]
        realname: Optional[str]
        firstname: Optional[str]
        email: Optional[str]

    class Group(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        name: Optional[str]
        completename: Optional[str]

    class ITILCategory(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        name: Optional[str]
        completename: Optional[str]

    class Location(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        name: Optional[str]
        completename: Optional[str]

    class Supplier(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        name: Optional[str]

    class RequestType(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        name: Optional[str]

    class TicketLabelRecord(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        label_name: Optional[str]

    class TicketCustomField(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        tickets_id: Optional[int]
        field_id: Optional[str]
        field_value: Optional[str]

    class TicketLink(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        tickets_id: Optional[int]
        tickets_id_1: Optional[int]
        tickets_id_2: Optional[int]
        link: Optional[int]

    class ITILSolution(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        tickets_id: Optional[int]
        content: Optional[str]

    b.create_tables(
        Connection,
        Entity,
        ScopeConfig,
        Ticket,
        TicketUser,
        TicketGroup,
        Followup,
        TicketTask,
        TicketLog,
        User,
        Group,
        ITILCategory,
        Location,
        Supplier,
        RequestType,
        TicketLabelRecord,
        TicketCustomField,
        TicketLink,
        ITILSolution,
    )
