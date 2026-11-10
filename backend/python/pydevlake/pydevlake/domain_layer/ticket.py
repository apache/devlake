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
    description: Optional[str] = None
    url: Optional[str] = None
    created_date: Optional[datetime] = None
    type: Optional[str] = None


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
    url: Optional[str] = None
    status: Optional[str] = None
    started_date: Optional[datetime] = None
    ended_date: Optional[datetime] = None
    completed_date: Optional[datetime] = None
    original_board_id: Optional[str] = None


class SprintIssue(NoPKModel, table=True):
    __tablename__ = 'sprint_issues'
    sprint_id: str = Field(primary_key=True)
    issue_id: str = Field(primary_key=True)


class Issue(DomainModel, table=True):
    __tablename__ = 'issues'
    url: Optional[str] = None
    issue_key: Optional[str] = None
    title: str
    description: Optional[str] = None
    epic_key: Optional[str] = None
    type: Optional[str] = None
    original_type: Optional[str] = None
    status: str
    original_status: Optional[str] = None
    story_point: Optional[float] = None
    resolution_date: Optional[datetime] = None
    created_date: Optional[datetime] = None
    updated_date: Optional[datetime] = None
    lead_time_minutes: Optional[int] = None
    original_estimate_minutes: Optional[int] = None
    time_spent_minutes: Optional[int] = None
    time_remaining_minutes: Optional[int] = None
    creator_id: Optional[str] = None
    creator_name: Optional[str] = None
    assignee_id: Optional[str] = None
    assignee_name: Optional[str] = None
    parent_issue_id: Optional[str] = None
    priority: Optional[str] = None
    severity: Optional[str] = None
    urgency: Optional[str] = None
    component: Optional[str] = None
    icon_url: Optional[str] = None
    original_project: Optional[str] = None
    is_subtask: bool = False
    due_date: Optional[datetime] = None
    fix_versions: Optional[str] = None


class IssueAssignee(NoPKModel, table=True):
    __tablename__ = 'issue_assignees'
    issue_id: str = Field(primary_key=True)
    assignee_id: str = Field(primary_key=True)
    assignee_name: Optional[str] = None


class IssueLabel(NoPKModel, table=True):
    __tablename__ = 'issue_labels'
    issue_id: str = Field(primary_key=True)
    label_name: str = Field(primary_key=True)


class IssueComment(DomainModel, table=True):
    __tablename__ = 'issue_comments'
    issue_id: str
    body: Optional[str] = None
    account_id: Optional[str] = None
    created_date: Optional[datetime] = None
    updated_date: Optional[datetime] = None


class IssueChangelogs(DomainModel, table=True):
    __tablename__ = 'issue_changelogs'
    issue_id: str
    author_id: Optional[str] = None
    author_name: Optional[str] = None
    field_id: Optional[str] = None
    field_name: Optional[str] = None
    original_from_value: Optional[str] = None
    original_to_value: Optional[str] = None
    from_value: Optional[str] = None
    to_value: Optional[str] = None
    created_date: Optional[datetime] = None


class IssueWorklog(DomainModel, table=True):
    __tablename__ = 'issue_worklogs'
    author_id: Optional[str] = None
    comment: Optional[str] = None
    time_spent_minutes: Optional[int] = None
    logged_date: Optional[datetime] = None
    started_date: Optional[datetime] = None
    issue_id: Optional[str] = None


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
