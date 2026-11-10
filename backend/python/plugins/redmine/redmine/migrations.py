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
from typing import Dict, Optional

from pydantic import AnyUrl, SecretStr

from pydevlake import Field
from pydevlake.migration import migration, MigrationScriptBuilder
from pydevlake.model import Connection as BaseConnection, Model, ToolModel, ToolScope, ToolTable


@migration(20260609000001, name="initialize schemas for Redmine")
def init_schemas(b: MigrationScriptBuilder):
    class Connection(BaseConnection):
        endpoint: AnyUrl
        token: SecretStr

    class Project(ToolScope):
        identifier: Optional[str] = None
        description: Optional[str] = None

    class ScopeConfig(ToolTable, Model):
        name: str = Field(default="default")
        entities: Optional[str] = None
        connection_id: Optional[int] = None
        collect_journals: bool = False
        collect_time_entries: bool = False
        tickets_created_after: Optional[datetime.datetime] = None
        type_mappings: Optional[Dict] = None
        status_mappings: Optional[Dict] = None

    class Issue(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        project_id: Optional[int] = None
        tracker_name: Optional[str] = None
        status_name: Optional[str] = None
        priority_name: Optional[str] = None
        author_id: Optional[int] = None
        author_name: Optional[str] = None
        assigned_to_id: Optional[int] = None
        assigned_to_name: Optional[str] = None
        parent_id: Optional[int] = None
        category_id: Optional[int] = None
        category_name: Optional[str] = None
        fixed_version_id: Optional[int] = None
        fixed_version_name: Optional[str] = None
        subject: Optional[str] = None
        description: Optional[str] = None
        start_date: Optional[datetime.date] = None
        due_date: Optional[datetime.date] = None
        done_ratio: Optional[int] = None
        estimated_hours: Optional[float] = None
        spent_hours: Optional[float] = None
        total_spent_hours: Optional[float] = None
        custom_fields_json: Optional[str] = None
        relations_json: Optional[str] = None
        created_on: Optional[datetime.datetime] = None
        updated_on: Optional[datetime.datetime] = None
        closed_on: Optional[datetime.datetime] = None

    class Journal(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        issue_id: Optional[int] = None
        user_id: Optional[int] = None
        user_name: Optional[str] = None
        notes: Optional[str] = None
        details_json: Optional[str] = None
        created_on: Optional[datetime.datetime] = None
        private_notes: Optional[bool] = None

    class TimeEntry(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        project_id: Optional[int] = None
        issue_id: Optional[int] = None
        user_id: Optional[int] = None
        user_name: Optional[str] = None
        hours: Optional[float] = None
        comments: Optional[str] = None
        spent_on: Optional[datetime.date] = None
        created_on: Optional[datetime.datetime] = None
        updated_on: Optional[datetime.datetime] = None

    b.create_tables(
        Connection,
        Project,
        ScopeConfig,
        Issue,
        Journal,
        TimeEntry,
    )
