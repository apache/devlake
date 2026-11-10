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


class PullRequest(DomainModel, table=True):
    __tablename__ = 'pull_requests'
    base_repo_id: str
    head_repo_id: str
    status: str
    original_status: str
    title: str
    description: str
    url: str
    author_name: str
    author_id: str
    parent_pr_id: Optional[str] = None
    pull_request_key: int
    created_date: datetime
    merged_date: Optional[datetime] = None
    closed_date: Optional[datetime] = None
    type: str
    component: str
    merge_commit_sha: str
    head_ref: str
    base_ref: str
    base_commit_sha: str
    head_commit_sha: str


class PullRequestLabels(NoPKModel, table=True):
    __tablename__ = 'pull_request_labels'
    pull_request_id: str = Field(primary_key=True)
    label_name: str = Field(primary_key=True)


class PullRequestCommit(NoPKModel, table=True):
    __tablename__ = 'pull_request_commits'
    commit_sha: str = Field(primary_key=True)
    pull_request_id: str = Field(primary_key=True)
    commit_author_name: str
    commit_author_email: str
    commit_authored_date: datetime


class PullRequestComment(DomainModel, table=True):
    __tablename__ = 'cicd_scopes'
    pull_request_id: str
    body: str
    account_id: str
    created_date: datetime
    commit_sha: str
    type: str
    review_id: str
    status: str


class Commit(NoPKModel, table=True):
    __tablename__ = 'commits'
    sha: str = Field(primary_key=True)
    additions: str
    deletions: Optional[str] = None
    dev_eq: Optional[str] = None
    message: str
    author_name: str
    author_email: int
    authored_date: datetime
    author_id: str
    committer_name: str
    committer_email: str
    committed_date: datetime
    committer_id: str


class CommitParent(NoPKModel, table=True):
    __tablename__ = 'commit_parents'
    commit_sha: str = Field(primary_key=True)
    parent_commit_sha: str = Field(primary_key=True)


class CommitsDiff(DomainModel, table=True):
    __tablename__ = 'commits_diffs'
    new_commit_sha: str = Field(primary_key=True)
    old_commit_sha: str = Field(primary_key=True)
    commit_sha: str = Field(primary_key=True)
    sorting_index: int


class RefCommit(NoPKModel, table=True):
    __tablename__ = 'ref_commits'
    new_ref_id: str = Field(primary_key=True)
    old_ref_id: str = Field(primary_key=True)
    new_commit_sha: str
    old_commit_sha: str


class Component(NoPKModel, table=True):
    __tablename__ = 'components'
    repo_id: str
    name: str = Field(primary_key=True)
    path_regex: str


class Ref(DomainModel, table=True):
    __tablename__ = "refs"
    repo_id: str
    name: str
    commit_sha: str
    is_default: bool
    ref_type: str
    created_date: datetime


class RefsPrCherryPick(DomainModel, table=True):
    __tablename__ = "refs_pr_cherrypicks"
    repo_name: str
    parent_pr_key: int
    cherrypick_base_branches: str
    cherrypick_pr_keys: str
    parent_pr_url: str
    parent_pr_id: str = Field(primary_key=True)


class Repo(DomainScope, table=True):
    __tablename__ = "repos"
    name: str
    url: str
    description: Optional[str] = None
    owner_id: Optional[str] = None
    language: Optional[str] = None
    forked_from: Optional[str] = None
    created_date: Optional[datetime] = None
    updated_date: Optional[datetime] = None
    deleted: bool


class RepoLanguage(NoPKModel, table=True):
    __tablename__ = "repo_languages"
    repo_id: str = Field(primary_key=True)
    language: str
    bytes: int


class RepoCommit(NoPKModel, table=True):
    __tablename__ = "repo_commits"
    repo_id: str = Field(primary_key=True)
    commit_sha: str = Field(primary_key=True)
