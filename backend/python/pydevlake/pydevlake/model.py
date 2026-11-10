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


import os
import json
from typing import Iterable, Optional
from inspect import getmodule
from datetime import datetime
from enum import Enum

import inflect
from pydantic import AnyUrl, SecretStr, field_validator, ConfigDict
from sqlalchemy import DateTime, Text
from sqlalchemy.orm import declared_attr
from sqlalchemy.inspection import inspect
from sqlmodel import SQLModel
from pydevlake import Field

inflect_engine = inflect.engine()


class Model(SQLModel):
    id: Optional[int] = Field(default=None, primary_key=True)
    created_at: Optional[datetime] = Field(
        default=None, sa_type=DateTime(), sa_column_kwargs={'default': datetime.utcnow}
    )
    updated_at: Optional[datetime] = Field(
        default=None, sa_type=DateTime(),
        sa_column_kwargs={'default': datetime.utcnow, 'onupdate': datetime.utcnow}
    )

    def set_updated_at(self):
        if self.updated_at is None:
            self.updated_at = datetime.utcnow()


def _to_camel(attr_name: str) -> str:
    """Convert snake_case to camelCase for alias generation."""
    parts = attr_name.split('_')
    return parts[0] + ''.join(word.capitalize() for word in parts[1:])


class ToolTable(SQLModel):
    model_config = ConfigDict(
        populate_by_name=True,
        alias_generator=_to_camel,
        # Pydantic v1 accepted numbers for str fields; APIs often return ids as numbers.
        coerce_numbers_to_str=True,
    )

    @declared_attr
    def __tablename__(cls) -> str:
        plugin_name = _get_plugin_name(cls)
        plural_entity = inflect_engine.plural_noun(cls.__name__.lower())
        return f'_tool_{plugin_name}_{plural_entity}'


class Connection(ToolTable, Model):
    name: str = Field(unique=True)
    proxy: Optional[AnyUrl] = None

    @field_validator('proxy', mode='before')
    @classmethod
    def allow_empty_proxy(cls, proxy):
        if proxy == "":
            return None
        return proxy


class DomainType(Enum):
    CODE = "CODE"
    TICKET = "TICKET"
    CODE_REVIEW = "CODEREVIEW"
    CROSS = "CROSS"
    CICD = "CICD"
    CODE_QUALITY = "CODEQUALITY"


class ScopeConfig(ToolTable, Model):
    name: str = Field(default="default")
    domain_types: list[DomainType] = Field(default=list(DomainType), alias="entities")
    connection_id: Optional[int] = None

    @field_validator('domain_types', mode='before')
    @classmethod
    def set_default_domain_types(cls, v):
        if v is None:
            return list(DomainType)
        return v


class RawModel(SQLModel):
    id: int = Field(primary_key=True)
    # Keep the column types of existing raw tables (they are only created if
    # missing): data/input are binary, url is TEXT.
    params: str = ''
    data: bytes
    url: str = Field(default='', sa_type=Text())
    input: bytes = b''
    created_at: datetime = Field(default_factory=datetime.now)


class RawDataOrigin(SQLModel):
    # SQLModel doesn't like attributes starting with _
    # so we change the names of the columns.
    raw_data_params: Optional[str] = Field(default=None, sa_column_kwargs={'name': '_raw_data_params'}, alias='_raw_data_params')
    raw_data_table: Optional[str] = Field(default=None, sa_column_kwargs={'name': '_raw_data_table'}, alias='_raw_data_table')
    raw_data_id: Optional[str] = Field(default=None, sa_column_kwargs={'name': '_raw_data_id'}, alias='_raw_data_id')
    raw_data_remark: Optional[str] = Field(default=None, sa_column_kwargs={'name': '_raw_data_remark'}, alias='_raw_data_remark')

    def set_raw_origin(self, raw: RawModel):
        self.raw_data_id = raw.id
        self.raw_data_params = raw.params
        self.raw_data_table = raw.__tablename__

    def set_tool_origin(self, tool_model: 'ToolModel'):
        self.raw_data_id = tool_model.raw_data_id
        self.raw_data_params = tool_model.raw_data_params
        self.raw_data_table = tool_model.raw_data_table


class NoPKModel(RawDataOrigin):
    created_at: Optional[datetime] = Field(
        default=None, sa_type=DateTime(), sa_column_kwargs={'default': datetime.utcnow}
    )
    updated_at: Optional[datetime] = Field(
        default=None, sa_type=DateTime(),
        sa_column_kwargs={'default': datetime.utcnow, 'onupdate': datetime.utcnow}
    )

    def set_updated_at(self):
        if self.updated_at is None:
            self.updated_at = datetime.utcnow()


class ToolModel(ToolTable, NoPKModel):
    connection_id: Optional[int] = Field(default=None, primary_key=True, auto_increment=False)

    def domain_id(self):
        """
        Generate an identifier for domain entities
        originates from self.
        """
        return domain_id(type(self), self.connection_id, *self.primary_keys())

    def primary_keys(self) -> Iterable[object]:
        model_type = type(self)
        mapper = inspect(model_type)
        for primary_key_column in mapper.primary_key:
            prop = mapper.get_property_by_column(primary_key_column)
            if prop.key == 'connection_id':
                continue
            yield getattr(self, prop.key)


class DomainModel(NoPKModel):
    id: Optional[str] = Field(default=None, primary_key=True)


class ToolScope(ToolModel):
    id: str = Field(primary_key=True)
    name: str
    scope_config_id: Optional[int] = None


class DomainScope(DomainModel):
    pass


def domain_id(model_type, connection_id, *args):
    """
    Generate an identifier for domain entities
    originates from a model of type model_type.
    """
    segments = [_get_plugin_name(model_type), model_type.__name__, str(connection_id)]
    segments.extend(str(arg) for arg in args)
    return ':'.join(segments)


def raw_data_params(connection_id: int, scope_id: str) -> str:
    # JSON keys MUST follow the Go conventions (CamelCase) and be sorted
    return json.dumps({
        "ConnectionId": connection_id,
        "ScopeId": scope_id
    }, separators=(',', ':'))


def _get_plugin_name(cls):
    """
    Get the plugin name from a class by looking into
    the file path of its module.
    """
    module = getmodule(cls)
    path_segments = module.__file__.split(os.sep)
    # Finds the name of the first enclosing folder
    # that is not a python module
    depth = len(module.__name__.split('.')) + 1
    return path_segments[-depth]


class SubtaskRun(SQLModel, table=True):
    __tablename__ = '_pydevlake_subtask_runs'
    """
    Table storing information about the execution of subtasks.
    """
    id: Optional[int] = Field(default=None, primary_key=True)
    subtask_name: str
    connection_id: int
    started: datetime
    completed: Optional[datetime] = None
    state: str = Field(sa_type=Text())  # JSON encoded dict of atomic values
