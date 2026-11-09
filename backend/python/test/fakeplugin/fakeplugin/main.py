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

import json
from datetime import datetime

from fakeplugin.models import FakeConnection, FakeProject, FakeScopeConfig, FakePipeline
from pydevlake import Plugin, Stream, RemoteScopeGroup, DomainType, TestConnectionResult
from pydevlake.domain_layer.devops import CicdScope, CICDPipeline, CICDStatus, CICDResult, CICDType

VALID_TOKEN = "this_is_a_valid_token"


class FakePipelineStream(Stream):
    tool_model = FakePipeline
    domain_types = [DomainType.CICD]

    def collect(self, state, context):
        for p in self.generate_fake_pipelines():
            yield json.loads(p.model_dump_json()), {}

    def convert(self, pipeline: FakePipeline, ctx):
        env = ctx.scope_config.env
        yield CICDPipeline(
            name=pipeline.id,
            status=self.convert_status(pipeline.state),
            finished_date=pipeline.finished_at,
            result=self.convert_result(pipeline.state),
            duration_sec=self.duration(pipeline),
            environment=env,
            type=CICDType.BUILD
        )

    def convert_status(self, state: FakePipeline.State):
        if state == FakePipeline.State.FAILURE or state == FakePipeline.State.SUCCESS:
            return CICDStatus.DONE
        else:
            return CICDStatus.IN_PROGRESS

    def convert_result(self, state: FakePipeline.State):
        if state == FakePipeline.State.SUCCESS:
            return CICDResult.SUCCESS
        elif state == FakePipeline.State.FAILURE:
            return CICDResult.FAILURE
        else:
            return None

    def duration(self, pipeline: FakePipeline):
        if pipeline.finished_at:
            return (pipeline.finished_at - pipeline.started_at).seconds
        return None

    @classmethod
    def generate_fake_pipelines(cls) -> list[FakePipeline]:
        states = [FakePipeline.State.SUCCESS, FakePipeline.State.FAILURE, FakePipeline.State.PENDING]
        fake_pipelines = []
        for i in range(250):
            fake_pipelines.append(FakePipeline(
                id=str(i),
                state=states[i % len(states)],
                started_at=datetime(2023, 1, 10, 11, 0, 0, microsecond=i),
                finished_at=datetime(2023, 1, 10, 11, 3, 0, microsecond=i),
            ))
        return fake_pipelines


class FakePlugin(Plugin):
    @property
    def connection_type(self):
        return FakeConnection

    @property
    def tool_scope_type(self):
        return FakeProject

    @property
    def scope_config_type(self):
        return FakeScopeConfig

    def domain_scopes(self, project: FakeProject):
        project_name = "_".join(project.name.lower().split(" "))
        yield CicdScope(
            id="1",
            name=project.name,
            url=f"http://fake.org/api/project/{project_name}"
        )

    def remote_scopes(self, connection: FakeConnection, group_id: str):
        if group_id == 'group1':
            return [
                FakeProject(
                    id='p1',
                    name='Project 1',
                    url='http://fake.org/api/project/p1'
                )
            ]
        else:
            return []

    def remote_scope_groups(self, connection: FakeConnection):
        return [
            RemoteScopeGroup(
                id='group1',
                name='Group 1'
            )
        ]

    def test_connection(self, connection: FakeConnection):
        if connection.token.get_secret_value() != VALID_TOKEN:
            return TestConnectionResult(
                success=False,
                message="Invalid token",
                status=401
            )
        return TestConnectionResult(
            success=True,
            message="Connection successful",
            status=200
        )

    @property
    def streams(self):
        return [
            FakePipelineStream
        ]


import fakeplugin.migrations  # NEEDED

if __name__ == '__main__':
    FakePlugin.start()
