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
        identifier: Optional[str]
        description: Optional[str]

    class ScopeConfig(ToolTable, Model):
        name: str = Field(default="default")
        entities: Optional[str]
        connection_id: Optional[int]
        collect_journals: bool = False
        collect_time_entries: bool = False
        tickets_created_after: Optional[datetime.datetime] = None
        type_mappings: Optional[Dict] = None
        status_mappings: Optional[Dict] = None

    class Issue(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        project_id: Optional[int]
        tracker_name: Optional[str]
        status_name: Optional[str]
        priority_name: Optional[str]
        author_id: Optional[int]
        author_name: Optional[str]
        assigned_to_id: Optional[int]
        assigned_to_name: Optional[str]
        parent_id: Optional[int]
        category_id: Optional[int]
        category_name: Optional[str]
        fixed_version_id: Optional[int]
        fixed_version_name: Optional[str]
        subject: Optional[str]
        description: Optional[str]
        start_date: Optional[datetime.date]
        due_date: Optional[datetime.date]
        done_ratio: Optional[int]
        estimated_hours: Optional[float]
        spent_hours: Optional[float]
        total_spent_hours: Optional[float]
        custom_fields_json: Optional[str]
        relations_json: Optional[str]
        created_on: Optional[datetime.datetime]
        updated_on: Optional[datetime.datetime]
        closed_on: Optional[datetime.datetime]

    class Journal(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        issue_id: Optional[int]
        user_id: Optional[int]
        user_name: Optional[str]
        notes: Optional[str]
        details_json: Optional[str]
        created_on: Optional[datetime.datetime]
        private_notes: Optional[bool]

    class TimeEntry(ToolModel):
        id: int = Field(primary_key=True, auto_increment=False)
        project_id: Optional[int]
        issue_id: Optional[int]
        user_id: Optional[int]
        user_name: Optional[str]
        hours: Optional[float]
        comments: Optional[str]
        spent_on: Optional[datetime.date]
        created_on: Optional[datetime.datetime]
        updated_on: Optional[datetime.datetime]

    b.create_tables(
        Connection,
        Project,
        ScopeConfig,
        Issue,
        Journal,
        TimeEntry,
    )
