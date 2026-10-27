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

from datetime import datetime
from typing import Optional

from sqlmodel import Field

from pydevlake.model import DomainModel, DomainScope, NoPKModel


class Board(DomainScope, table=True):
    __tablename__ = 'boards'
    name: str
    description: Optional[str]
    url: Optional[str]
    created_date: Optional[datetime]
    type: Optional[str]


class BoardIssue(NoPKModel, table=True):
    __tablename__ = 'board_issues'
    board_id: str = Field(primary_key=True)
    issue_id: str = Field(primary_key=True)


class BoardSprint(NoPKModel, table=True):
    __tablename__ = 'board_sprints'
    board_id: str = Field(primary_key=True)
    sprint_id: str = Field(primary_key=True)


class Sprint(DomainModel, table=True):
    __tablename__ = 'sprints'
    name: str
    url: Optional[str]
    status: Optional[str]
    started_date: Optional[datetime]
    ended_date: Optional[datetime]
    completed_date: Optional[datetime]
    original_board_id: Optional[str]


class SprintIssue(NoPKModel, table=True):
    __tablename__ = 'sprint_issues'
    sprint_id: str = Field(primary_key=True)
    issue_id: str = Field(primary_key=True)


class Issue(DomainModel, table=True):
    __tablename__ = 'issues'
    url: Optional[str]
    issue_key: Optional[str]
    title: str
    description: Optional[str]
    epic_key: Optional[str]
    type: Optional[str]
    original_type: Optional[str]
    status: str
    original_status: Optional[str]
    story_point: Optional[float]
    resolution_date: Optional[datetime]
    created_date: Optional[datetime]
    updated_date: Optional[datetime]
    lead_time_minutes: Optional[int]
    original_estimate_minutes: Optional[int]
    time_spent_minutes: Optional[int]
    time_remaining_minutes: Optional[int]
    creator_id: Optional[str]
    creator_name: Optional[str]
    assignee_id: Optional[str]
    assignee_name: Optional[str]
    parent_issue_id: Optional[str]
    priority: Optional[str]
    severity: Optional[str]
    urgency: Optional[str]
    component: Optional[str]
    icon_url: Optional[str]
    original_project: Optional[str]
    is_subtask: bool = False
    due_date: Optional[datetime]
    fix_versions: Optional[str]


class IssueAssignee(NoPKModel, table=True):
    __tablename__ = 'issue_assignees'
    issue_id: str = Field(primary_key=True)
    assignee_id: str = Field(primary_key=True)
    assignee_name: Optional[str]


class IssueLabel(NoPKModel, table=True):
    __tablename__ = 'issue_labels'
    issue_id: str = Field(primary_key=True)
    label_name: str = Field(primary_key=True)


class IssueComment(DomainModel, table=True):
    __tablename__ = 'issue_comments'
    issue_id: str
    body: Optional[str]
    account_id: Optional[str]
    created_date: Optional[datetime]
    updated_date: Optional[datetime]


class IssueChangelogs(DomainModel, table=True):
    __tablename__ = 'issue_changelogs'
    issue_id: str
    author_id: Optional[str]
    author_name: Optional[str]
    field_id: Optional[str]
    field_name: Optional[str]
    original_from_value: Optional[str]
    original_to_value: Optional[str]
    from_value: Optional[str]
    to_value: Optional[str]
    created_date: Optional[datetime]


class IssueWorklog(DomainModel, table=True):
    __tablename__ = 'issue_worklogs'
    author_id: Optional[str]
    comment: Optional[str]
    time_spent_minutes: Optional[int]
    logged_date: Optional[datetime]
    started_date: Optional[datetime]
    issue_id: Optional[str]


class IssueRelationship(NoPKModel, table=True):
    __tablename__ = 'issue_relationships'
    source_issue_id: str = Field(primary_key=True)
    target_issue_id: str = Field(primary_key=True)
    original_type: str = Field(primary_key=True)


class IssueCustomArrayField(NoPKModel, table=True):
    __tablename__ = 'issue_custom_array_fields'
    issue_id: str = Field(primary_key=True)
    field_id: str = Field(primary_key=True)
    field_value: str = Field(primary_key=True)
