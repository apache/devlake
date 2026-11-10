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


from typing import Literal, Optional

from pydantic import BaseModel, ConfigDict, Field, RootModel, SerializeAsAny

from pydevlake.model import ToolScope
from pydevlake.migration import MigrationScript
from pydevlake.api import Response
from pydevlake.model_info import DynamicModelInfo


class Message(BaseModel):
    model_config = ConfigDict(populate_by_name=True)


class SubtaskMeta(BaseModel):
    name: str
    entry_point_name: str
    required: bool
    enabled_by_default: bool
    description: str
    domain_types: list[str]
    arguments: Optional[list[str]] = None


class PluginInfo(Message):
    name: str
    description: str
    connection_model_info: DynamicModelInfo
    scope_model_info: DynamicModelInfo
    scope_config_model_info: Optional[DynamicModelInfo] = None
    tool_model_infos: list[DynamicModelInfo]
    migration_scripts: list[MigrationScript]
    plugin_path: str
    subtask_metas: list[SubtaskMeta]
    extension: str = "datasource"


class RemoteProgress(Message):
    increment: int = 0
    current: int = 0
    total: int = 0


class PipelineTask(Message):
    plugin: str
    skip_on_fail: bool = Field(default=False, alias="skipOnFail")
    subtasks: list[str] = Field(default_factory=list)
    options: dict[str, object] = Field(default_factory=dict)


class DynamicDomainScope(Message):
    type_name: str
    data: bytes


class PipelineData(Message):
    plan: list[list[PipelineTask]]
    scopes: list[DynamicDomainScope]


class RemoteScopeTreeNode(Message):
    id: str
    name: str


class RemoteScopeGroup(RemoteScopeTreeNode):
    type: Literal["group"] = "group"


class RemoteScope(RemoteScopeTreeNode):
    type: Literal["scope"] = "scope"
    parent_id: str = Field(..., alias="parentId")
    # SerializeAsAny keeps pydantic v2 from serializing this field by its
    # declared base type (ToolScope) and dropping subclass-only fields such as
    # the concrete scope's `url`; v1 always serialized the runtime type.
    data: SerializeAsAny[ToolScope]


class RemoteScopes(RootModel[list[RemoteScopeTreeNode]]):
    pass


class TestConnectionResult(Message):
    success: bool
    message: str
    status: int

    @staticmethod
    def from_api_response(response: Response, message: str = None):
        success = response.status == 200
        if not message:
            message = "Connection successful" if success else "Connection failed"
        return TestConnectionResult(
            success=success,
            message=message,
            status=response.status
        )
