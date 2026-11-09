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

from pydantic import AnyUrl, BaseModel, ConfigDict, SecretStr

from pydevlake import Field, ScopeConfig as BaseScopeConfig
from pydevlake.model import Connection as BaseConnection, ToolModel, ToolScope

from redmine.migrations import *


class StatusMapping(BaseModel):
    standard_status: str = Field(alias='standardStatus')

    model_config = ConfigDict(populate_by_name=True)


class TypeMapping(BaseModel):
    standard_type: str = Field(alias='standardType')

    model_config = ConfigDict(populate_by_name=True)


class Connection(BaseConnection):
    endpoint: AnyUrl
    token: SecretStr


class ScopeConfig(BaseScopeConfig):
    collect_journals: bool = False
    collect_time_entries: bool = False
    type_mappings: Dict[str, TypeMapping] = Field(default_factory=dict, alias='typeMappings')
    status_mappings: Dict[str, StatusMapping] = Field(default_factory=dict, alias='statusMappings')


class Project(ToolScope, table=True):
    identifier: Optional[str] = None
    description: Optional[str] = None


class Issue(ToolModel, table=True):
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


class Journal(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    issue_id: Optional[int] = None
    user_id: Optional[int] = None
    user_name: Optional[str] = None
    notes: Optional[str] = None
    details_json: Optional[str] = None
    created_on: Optional[datetime.datetime] = None
    private_notes: Optional[bool] = None


class TimeEntry(ToolModel, table=True):
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
