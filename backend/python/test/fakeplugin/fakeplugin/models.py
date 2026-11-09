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
from enum import Enum
from typing import Optional

from pydantic import SecretStr

from pydevlake import ScopeConfig, ToolScope, Connection, ToolModel, Field


class FakeConnection(Connection):
    token: SecretStr


class FakeProject(ToolScope, table=True):
    url: str


class FakeScopeConfig(ScopeConfig):
    env: str


class FakePipeline(ToolModel, table=True):
    class State(Enum):
        PENDING = "pending"
        RUNNING = "running"
        FAILURE = "failure"
        SUCCESS = "success"

    id: str = Field(primary_key=True)
    started_at: Optional[datetime] = None
    finished_at: Optional[datetime] = None
    state: State
