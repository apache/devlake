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
import re
from enum import Enum
from typing import Optional

from pydantic import SecretStr

from pydevlake import ToolModel, Connection, Field
from pydevlake.migration import migration, Dialect, MigrationScriptBuilder
from pydevlake.model import Model, ToolTable
from pydevlake.pipeline_tasks import RefDiffOptions


@migration(20230501000001, name="initialize schemas for Azure Devops")
def init_schemas(b: MigrationScriptBuilder):
    class AzureDevOpsConnection(Connection):
        token: SecretStr
        organization: Optional[str] = None

    class AzureDevopsTransformationRule(ToolTable, Model):
        name: str = Field(default="default")
        refdiff: Optional[RefDiffOptions] = None
        deployment_pattern: Optional[re.Pattern] = None
        production_pattern: Optional[re.Pattern] = None

    class GitRepository(ToolModel):
        id: str = Field(primary_key=True)
        name: str
        url: str
        remote_url: Optional[str] = None
        default_branch: Optional[str] = None
        project_id: str
        org_id: str
        parent_repository_url: Optional[str] = None
        provider: Optional[str] = None

    class GitPullRequest(ToolModel):
        class PRStatus(Enum):
            Abandoned = "abandoned"
            Active = "active"
            Completed = "completed"

        pull_request_id: int = Field(primary_key=True)
        description: Optional[str] = None
        status: PRStatus
        created_by_id: str
        created_by_name: str
        creation_date: datetime.datetime
        closed_date: Optional[datetime.datetime] = None
        source_commit_sha: str
        target_commit_sha: str
        merge_commit_sha: Optional[str] = None
        url: Optional[str] = None
        type: Optional[str] = None
        title: Optional[str] = None
        target_ref_name: Optional[str] = None
        source_ref_name: Optional[str] = None
        fork_repo_id: Optional[str] = None

    class GitPullRequestCommit(ToolModel):
        commit_id: str = Field(primary_key=True)
        pull_request_id: str
        author_name: str
        author_email: str
        author_date: datetime.datetime

    class Build(ToolModel):
        class BuildStatus(Enum):
            Cancelling = "cancelling"
            Completed = "completed"
            InProgress = "inProgress"
            NotStarted = "notStarted"
            Postponed = "postponed"

        class BuildResult(Enum):
            Canceled = "canceled"
            Failed = "failed"
            Non = "none"
            PartiallySucceeded = "partiallySucceeded"
            Succeeded = "succeeded"

        id: int = Field(primary_key=True, auto_increment=False)
        name: str
        start_time: Optional[datetime.datetime] = None
        finish_time: Optional[datetime.datetime] = None
        status: BuildStatus
        result: Optional[BuildResult] = None
        source_branch: str
        source_version: str

    class Job(ToolModel):
        class JobState(Enum):
            Completed = "completed"
            InProgress = "inProgress"
            Pending = "pending"

        class JobResult(Enum):
            Abandoned = "abandoned"
            Canceled = "canceled"
            Failed = "failed"
            Skipped = "skipped"
            Succeeded = "succeeded"
            SucceededWithIssues = "succeededWithIssues"

        id: str = Field(primary_key=True)
        build_id: str
        name: str
        startTime: Optional[datetime.datetime] = None
        finishTime: Optional[datetime.datetime] = None
        state: JobState
        result: Optional[JobResult] = None

    b.create_tables(
        AzureDevOpsConnection,
        AzureDevopsTransformationRule,
        GitRepository,
        GitPullRequestCommit,
        GitPullRequest,
        Build,
        Job,
    )


@migration(20230524181430)
def add_build_id_as_job_primary_key(b: MigrationScriptBuilder):
    # NOTE: We can't add a column to the primary key of an existing table,
    # so we have to drop the primary key constraint first,
    # which is done differently in MySQL and PostgreSQL,
    # and then add the new composite primary key.
    table = '_tool_azuredevops_jobs'
    b.execute(f'ALTER TABLE {table} MODIFY COLUMN build_id VARCHAR(255)', Dialect.MYSQL)
    b.execute(f'ALTER TABLE {table} ALTER COLUMN build_id TYPE VARCHAR(255)', Dialect.POSTGRESQL)
    b.execute(f'ALTER TABLE {table} DROP PRIMARY KEY', Dialect.MYSQL)
    b.execute(f'ALTER TABLE {table} DROP CONSTRAINT {table}_pkey', Dialect.POSTGRESQL)
    b.execute(f'ALTER TABLE {table} ADD PRIMARY KEY (id, build_id)')


@migration(20230606165630)
def rename_tx_rule_table_to_scope_config(b: MigrationScriptBuilder):
    b.rename_table('_tool_azuredevops_azuredevopstransformationrules', '_tool_azuredevops_gitrepositoryconfigs')


@migration(20230607165630, name="add entities column to gitrepositoryconfig table")
def add_entities_column_to_scope_config(b: MigrationScriptBuilder):
    b.add_column('_tool_azuredevops_gitrepositoryconfigs', 'entities', 'json')


@migration(20230630000001, name="populated _raw_data_table column for azuredevops git repos")
def add_raw_data_params_table_to_scope(b: MigrationScriptBuilder):
    b.execute(f'''UPDATE _tool_azuredevops_gitrepositories SET _raw_data_table = '_raw_azuredevops_scopes' WHERE 1=1''')


@migration(20230802000001, name="rename startTime/finishTime to start_time/finish_time")
def rename_starttime_and_finishtime_for_job(b: MigrationScriptBuilder):
    b.rename_column('_tool_azuredevops_jobs', 'startTime', 'start_time')
    b.rename_column('_tool_azuredevops_jobs', 'finishTime', 'finish_time')


@migration(20230825150421, name="add missing migrations from 0.17 to 0.18")
def add_missing_migrations_0_17_to_0_18(b: MigrationScriptBuilder):
    b.rename_column('_tool_azuredevops_gitrepositories', 'transformation_rule_id', 'scope_config_id')
    b.add_column('_tool_azuredevops_gitrepositories', 'provider', 'varchar(255)')


@migration(20231013130200, name="add missing field in _tool_azuredevops_gitrepositoryconfigs")
def add_missing_field_in_tool_azuredevops_gitrepositoryconfigs(b: MigrationScriptBuilder):
    b.add_column('_tool_azuredevops_gitrepositoryconfigs', 'connection_id', 'bigint')


@migration(20231013130201, name="add missing field in _tool_azuredevops_gitrepositories")
def add_missing_field_in_tool_azuredevops_gitrepositories(b: MigrationScriptBuilder):
    b.add_column('_tool_azuredevops_gitrepositories', 'scope_config_id', 'bigint')


@migration(20231130163000, name="add queue_time field in _tool_azuredevops_builds")
def add_queue_time_field_in_tool_azuredevops_builds(b: MigrationScriptBuilder):
    table = '_tool_azuredevops_builds'
    b.execute(f'ALTER TABLE {table} ADD COLUMN queue_time timestamptz', Dialect.POSTGRESQL)
    b.execute(f'ALTER TABLE {table} ADD COLUMN queue_time datetime', Dialect.MYSQL)


@migration(20240223170000, name="add updated_date field in _tool_azuredevops_gitrepositories")
def add_updated_date_field_in_tool_azuredevops_gitrepositories(b: MigrationScriptBuilder):
    table = "_tool_azuredevops_gitrepositories"
    b.execute(f'ALTER TABLE {table} add COLUMN updated_date datetime(3) DEFAULT NULL', Dialect.MYSQL)
    b.execute(f'ALTER TABLE {table} add COLUMN updated_date TIMESTAMPTZ DEFAULT NULL', Dialect.POSTGRESQL)

@migration(20240415163000, name="add queue_time field in _tool_azuredevops_builds")
def add_queue_time_field_in_tool_azuredevops_builds(b: MigrationScriptBuilder):
    table = '_tool_azuredevops_builds'
    b.add_column(table, 'display_title', 'TEXT')
    b.add_column(table, 'url', 'TEXT')

@migration(20240521133000, name="Update url column in _tool_azuredevops_builds")
def add_updated_url_column_length_in_tool_azuredevops_builds(b: MigrationScriptBuilder):
    table = '_tool_azuredevops_builds'
    b.execute(f'ALTER TABLE {table} MODIFY COLUMN url TEXT', Dialect.MYSQL)
    b.execute(f'ALTER TABLE {table} ALTER COLUMN url TYPE TEXT', Dialect.POSTGRESQL)

@migration(20240527113400, name="Update url column in _raw_azuredevops_builds ")
def add_updated_url_column_length_in_raw_azuredevops_builds(b: MigrationScriptBuilder):
    table = '_raw_azuredevops_builds'
    
    b.execute(f"""
    CREATE PROCEDURE alter_url_column_if_exists()
    BEGIN
        DECLARE table_exists INT DEFAULT 0;
        SELECT COUNT(*) INTO table_exists
        FROM information_schema.tables 
        WHERE table_schema = DATABASE() 
          AND table_name = '{table}';
        IF table_exists > 0 THEN
            ALTER TABLE {table} MODIFY COLUMN url TEXT;
        ELSE
            SELECT 'Table {table} does not exist' AS message;
        END IF;
    END;
    """, Dialect.MYSQL)
    
    b.execute(f"CALL alter_url_column_if_exists();", Dialect.MYSQL)
    b.execute(f"DROP PROCEDURE alter_url_column_if_exists;", Dialect.MYSQL)
    
    b.execute(f"""
    DO $$
    BEGIN
        IF EXISTS (SELECT FROM information_schema.tables 
                   WHERE table_schema = 'public' AND table_name = '{table}') THEN
            ALTER TABLE {table} ALTER COLUMN url TYPE TEXT;
        ELSE
            RAISE NOTICE 'Table {table} does not exist';
        END IF;
    END $$;
    """, Dialect.POSTGRESQL)

