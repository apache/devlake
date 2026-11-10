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


class Project(ToolScope, table=True):
    jira_project_id: Optional[int] = None
    project_key: Optional[str] = None


class Folder(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    project_key: Optional[str] = None
    name: Optional[str] = None
    parent_id: Optional[int] = None
    folder_type: Optional[str] = None
    index: Optional[int] = None


class TestCase(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    key: Optional[str] = None
    project_key: Optional[str] = None
    name: Optional[str] = None
    objective: Optional[str] = None
    precondition: Optional[str] = None
    created_on: Optional[datetime.datetime] = None
    folder_id: Optional[int] = None
    folder_name: Optional[str] = None
    component_id: Optional[int] = None
    component_name: Optional[str] = None
    priority_id: Optional[int] = None
    priority_name: Optional[str] = None
    status_id: Optional[int] = None
    status_name: Optional[str] = None
    owner_account_id: Optional[str] = None
    estimated_time: Optional[int] = None
    labels_json: Optional[str] = None
    custom_fields_json: Optional[str] = None
    issue_links_json: Optional[str] = None


class TestCaseLabel(ToolModel, table=True):
    id: str = Field(primary_key=True, auto_increment=False)
    test_case_id: Optional[int] = None
    test_case_key: Optional[str] = None
    label_name: Optional[str] = None


class TestCaseIssueLink(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    test_case_id: Optional[int] = None
    test_case_key: Optional[str] = None
    issue_id: Optional[int] = None
    link_type: Optional[str] = None


class TestCycle(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    key: Optional[str] = None
    project_key: Optional[str] = None
    name: Optional[str] = None
    description: Optional[str] = None
    folder_id: Optional[int] = None
    folder_name: Optional[str] = None
    status_id: Optional[int] = None
    status_name: Optional[str] = None
    jira_project_version_id: Optional[int] = None
    jira_project_version_name: Optional[str] = None
    planned_start_date: Optional[datetime.datetime] = None
    planned_end_date: Optional[datetime.datetime] = None


class TestExecution(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    key: Optional[str] = None
    project_key: Optional[str] = None
    test_case_id: Optional[int] = None
    test_cycle_id: Optional[int] = None
    test_execution_status_id: Optional[int] = None
    test_execution_status_name: Optional[str] = None
    jira_project_version_id: Optional[int] = None
    jira_project_version_name: Optional[str] = None
    actual_end_date: Optional[datetime.datetime] = None
    execution_time: Optional[int] = None
    estimated_time: Optional[int] = None
    executed_by_id: Optional[str] = None
    assigned_to_id: Optional[str] = None
    comment: Optional[str] = None
    automated: Optional[bool] = None
    custom_fields_json: Optional[str] = None
    issue_links_json: Optional[str] = None


class TestExecutionIssueLink(ToolModel, table=True):
    id: int = Field(primary_key=True, auto_increment=False)
    test_execution_id: Optional[int] = None
    test_execution_key: Optional[str] = None
    issue_id: Optional[int] = None
    link_type: Optional[str] = None
