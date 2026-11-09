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
        completename: Optional[str] = None

    class ScopeConfig(ToolTable, Model):
        name: str = Field(default="default")
        entities: Optional[str] = None
        connection_id: Optional[int] = None
        collect_assignees: bool = True
        collect_followups: bool = False
        collect_ticket_links: bool = False
        collect_logs: bool = False
        collect_tasks: bool = False
        collect_solutions: bool = False
        tickets_created_after: Optional[datetime.datetime] = None

    class Ticket(ToolModel):
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

    class TicketUser(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        tickets_id: Optional[int] = None
        users_id: Optional[int] = None
        user_name: Optional[str] = None
        type: Optional[int] = None

    class TicketGroup(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        tickets_id: Optional[int] = None
        groups_id: Optional[int] = None
        group_name: Optional[str] = None
        type: Optional[int] = None

    class Followup(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        items_id: Optional[int] = None
        users_id: Optional[int] = None
        content: Optional[str] = None
        date: Optional[datetime.datetime] = None
        date_mod: Optional[datetime.datetime] = None
        is_private: Optional[int] = None

    class TicketTask(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        tickets_id: Optional[int] = None
        users_id: Optional[int] = None
        content: Optional[str] = None
        begin: Optional[datetime.datetime] = None
        end: Optional[datetime.datetime] = None
        actiontime: Optional[int] = None

    class TicketLog(ToolModel):
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

    class User(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        name: Optional[str] = None
        realname: Optional[str] = None
        firstname: Optional[str] = None
        email: Optional[str] = None

    class Group(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        name: Optional[str] = None
        completename: Optional[str] = None

    class ITILCategory(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        name: Optional[str] = None
        completename: Optional[str] = None

    class Location(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        name: Optional[str] = None
        completename: Optional[str] = None

    class Supplier(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        name: Optional[str] = None

    class RequestType(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        name: Optional[str] = None

    class TicketLabelRecord(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        label_name: Optional[str] = None

    class TicketCustomField(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        tickets_id: Optional[int] = None
        field_id: Optional[str] = None
        field_value: Optional[str] = None

    class TicketLink(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        tickets_id: Optional[int] = None
        tickets_id_1: Optional[int] = None
        tickets_id_2: Optional[int] = None
        link: Optional[int] = None

    class ITILSolution(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        tickets_id: Optional[int] = None
        content: Optional[str] = None

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
