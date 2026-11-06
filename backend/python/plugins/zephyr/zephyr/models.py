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

from pydevlake import Field, ScopeConfig as BaseScopeConfig
from pydevlake.model import Connection as BaseConnection, ToolModel, ToolScope

from zephyr.migrations import *

DEFAULT_API_BASE = 'https://api.zephyrscale.smartbear.com/v2'


class Connection(BaseConnection):
    endpoint: AnyUrl = DEFAULT_API_BASE
    token: SecretStr


class ScopeConfig(BaseScopeConfig):
    test_type_custom_field: str = Field(default='Test Type', alias='testTypeCustomField')

    class Config:
        allow_population_by_field_name = True


class Project(ToolScope, table=True):
    jira_project_id: Optional[int]
    project_key: Optional[str]


class Folder(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    project_key: Optional[str]
    name: Optional[str]
    parent_id: Optional[int]
    folder_type: Optional[str]
    index: Optional[int]


class TestCase(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    key: Optional[str]
    project_key: Optional[str]
    name: Optional[str]
    objective: Optional[str]
    precondition: Optional[str]
    created_on: Optional[datetime.datetime]
    folder_id: Optional[int]
    folder_name: Optional[str]
    component_id: Optional[int]
    component_name: Optional[str]
    priority_id: Optional[int]
    priority_name: Optional[str]
    status_id: Optional[int]
    status_name: Optional[str]
    owner_account_id: Optional[str]
    estimated_time: Optional[int]
    labels_json: Optional[str]
    custom_fields_json: Optional[str]
    issue_links_json: Optional[str]


class TestCaseLabel(ToolModel, table=True):
    id: str = Field(primary_key=True, auto_increment=False)
    test_case_id: Optional[int]
    test_case_key: Optional[str]
    label_name: Optional[str]


class TestCaseIssueLink(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    test_case_id: Optional[int]
    test_case_key: Optional[str]
    issue_id: Optional[int]
    link_type: Optional[str]


class TestCycle(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    key: Optional[str]
    project_key: Optional[str]
    name: Optional[str]
    description: Optional[str]
    folder_id: Optional[int]
    folder_name: Optional[str]
    status_id: Optional[int]
    status_name: Optional[str]
    jira_project_version_id: Optional[int]
    jira_project_version_name: Optional[str]
    planned_start_date: Optional[datetime.datetime]
    planned_end_date: Optional[datetime.datetime]


class TestExecution(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    key: Optional[str]
    project_key: Optional[str]
    test_case_id: Optional[int]
    test_cycle_id: Optional[int]
    test_execution_status_id: Optional[int]
    test_execution_status_name: Optional[str]
    jira_project_version_id: Optional[int]
    jira_project_version_name: Optional[str]
    actual_end_date: Optional[datetime.datetime]
    execution_time: Optional[int]
    estimated_time: Optional[int]
    executed_by_id: Optional[str]
    assigned_to_id: Optional[str]
    comment: Optional[str]
    automated: Optional[bool]
    custom_fields_json: Optional[str]
    issue_links_json: Optional[str]


class TestExecutionIssueLink(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    test_execution_id: Optional[int]
    test_execution_key: Optional[str]
    issue_id: Optional[int]
    link_type: Optional[str]
