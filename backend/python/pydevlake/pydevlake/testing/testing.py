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

import pytest

from typing import Union, Type, Iterable, Generator, Optional

from pydantic import ValidationError
from sqlmodel import create_engine

from pydevlake.context import Context
from pydevlake.plugin import Plugin
from pydevlake.message import RemoteScopeGroup, PipelineTask
from pydevlake.model import DomainModel, Connection, DomainScope, ToolModel, ToolScope, ScopeConfig, DomainType
from pydevlake.stream import Stream


class ContextBuilder:
    def __init__(self, plugin: Plugin):
        if isinstance(plugin, type):
            plugin = plugin()
        self.plugin = plugin
        self.connection = None
        self.scope = None
        self.scope_config = None
        self.options = None
        self.time_after = None

    def with_connection(self, id=1, name='test_connection', **kwargs):
        self.connection = self.plugin.connection_type(id=id, name=name, **kwargs)
        return self

    def with_scope(self, id='s', name='test_scope', **kwargs):
        self.scope = self.plugin.tool_scope_type(id=id, name=name, **kwargs)
        if self.connection:
            self.scope.connection_id = self.connection.id
        return self

    def with_scope_config(self, id=1, name='test_config', **kwargs):
        self.scope_config = self.plugin.scope_config_type(id=id, name=name, **kwargs)
        return self

    def with_options(self, **kwargs):
        self.options = kwargs
        return self

    def with_time_after(self, time_after):
        self.time_after = time_after
        return self

    def build(self):
        return make_context(
            self.connection,
            self.scope,
            self.scope_config,
            options=self.options,
            time_after=self.time_after,
        )


def make_context(connection, scope, scope_config, options=None, time_after=None):
    return Context(
        engine=create_engine('sqlite:///:memory:'),
        scope=scope,
        connection=connection,
        scope_config=scope_config,
        options=options,
        time_after=time_after,
    )


def assert_stream_convert(plugin: Union[Plugin, Type[Plugin]], stream_name: str,
                   raw: dict, expected: Union[DomainModel, Iterable[DomainModel]],
                   ctx=None):
    if isinstance(plugin, type):
        plugin = plugin()
    stream = plugin.get_stream(stream_name)
    tool_model = stream.extract(raw)
    if ctx and ctx.connection:
        tool_model.connection_id = ctx.connection.id
    domain_models = stream.convert(tool_model, ctx)
    if not isinstance(expected, list):
        expected = [expected]
    if not isinstance(domain_models, (Iterable, Generator)):
        domain_models = [domain_models]
    for res, exp in zip(domain_models, expected):
        if (hasattr(res, "__getitem__") and res[0] == "updated_at") or (
                hasattr(exp, "__getitem__") and exp[0] == "updated_at"):
            pass
        else:
            assert res == exp


def assert_stream_run(stream: Stream, connection: Connection, scope: ToolScope, scope_config: ScopeConfig):
    """
    Test that a stream can run all 3 steps without error.
    """
    ctx = make_context(connection, scope, scope_config)
    stream.collector.run(ctx)
    stream.extractor.run(ctx)
    stream.convertor.run(ctx)


def assert_valid_name(plugin: Plugin):
    name = plugin.name
    assert isinstance(name, str), 'name must be a string'
    assert name.isalnum(), 'name must be alphanumeric'


def assert_valid_description(plugin: Plugin):
    name = plugin.description
    assert isinstance(name, str), 'description must be a string'


def assert_valid_connection_type(plugin: Plugin):
    connection_type = plugin.connection_type
    assert issubclass(connection_type, Connection), 'connection_type must be a subclass of Connection'


def assert_valid_tool_scope_type(plugin: Plugin):
    tool_scope_type = plugin.tool_scope_type
    assert issubclass(tool_scope_type, ToolScope), 'tool_scope_type must be a subclass of ToolScope'


def assert_valid_scope_config_type(plugin: Plugin):
    scope_config_type = plugin.scope_config_type
    assert issubclass(scope_config_type, ScopeConfig), 'scope_config_type must be a subclass of ScopeConfig'
    try:
        scope_config_type()
    except ValidationError as e:
        pytest.fail(f'scope_config_type must not have required fields: {e}')


def assert_valid_streams(plugin: Plugin):
    streams = plugin.streams
    assert isinstance(streams, list), 'streams must be a list'
    assert len(streams) > 0, 'this plugin has no stream'
    for stream in streams:
        if isinstance(stream, type):
            stream = stream(plugin.name)
        assert isinstance(stream, Stream), 'stream must be a stream class or instance'
        assert_valid_stream(stream)


def assert_valid_stream(stream: Stream):
    assert isinstance(stream.name, str), 'name must be a string'
    assert issubclass(stream.tool_model, ToolModel), 'tool_model must be a subclass of ToolModel'
    domain_types = stream.domain_types
    assert len(domain_types) > 0, 'stream must have at least one domain type'
    for domain_type in domain_types:
        assert isinstance(domain_type, DomainType), 'domain type must be a DomainType'


def assert_valid_connection(plugin: Plugin, connection: Connection):
    try:
        plugin.test_connection(connection)
    except Exception as e:
        pytest.fail(f'Connection is not valid: {e}')


def assert_valid_domain_scopes(plugin: Plugin, tool_scope: ToolScope) -> list[DomainScope]:
    domain_scopes = list(plugin.domain_scopes(tool_scope))
    assert len(domain_scopes) > 0, 'No domain scope generated for given tool scope'
    for domain_scope in domain_scopes:
        assert isinstance(domain_scope, DomainScope), 'Domain scope must be a DomainScope'
    return domain_scopes


def assert_valid_remote_scope_groups(plugin: Plugin, connection: Connection) -> list[RemoteScopeGroup]:
    scope_groups = list(plugin.remote_scope_groups(connection))
    assert len(scope_groups) > 0, 'This connection has no scope groups'
    for scope_group in scope_groups:
        assert isinstance(scope_group, RemoteScopeGroup), 'Scope group must be a RemoteScopeGroup'
        assert scope_group.id is not None, 'Scope group id must not be None'
        assert bool(scope_group.name), 'Scope group name must not be empty'
    return scope_groups


def assert_valid_remote_scopes(plugin: Plugin, connection: Connection, group_id: str) -> list[ToolScope]:
    tool_scopes = list(plugin.remote_scopes(connection, group_id))
    assert len(tool_scopes) > 0, 'This connection has no scopes'
    for tool_scope in tool_scopes:
        assert isinstance(tool_scope, ToolScope), 'Remote scope must be a ToolScope'
    return tool_scopes


def assert_valid_pipeline_plan(plugin: Plugin, connection: Connection, tool_scope: ToolScope, scope_config: ScopeConfig) -> list[list[PipelineTask]]:
    plan = plugin.make_pipeline_plan(
        [(tool_scope, scope_config)],
        connection
    )
    assert len(plan) > 0, 'Pipeline plan has no stage'
    for stage in plan:
        assert len(stage) > 0, 'Pipeline stage has no task'
    return plan


def assert_valid_plugin(plugin: Plugin):
    assert_valid_name(plugin)
    assert_valid_description(plugin)
    assert_valid_connection_type(plugin)
    assert_valid_tool_scope_type(plugin)
    assert_valid_scope_config_type(plugin)
    assert_valid_streams(plugin)


def assert_plugin_run(plugin: Plugin, connection: Connection, scope_config: Optional[ScopeConfig] = None):
    assert_valid_plugin(plugin)
    assert_valid_connection(plugin, connection)
    groups = assert_valid_remote_scope_groups(plugin, connection)
    scope = assert_valid_remote_scopes(plugin, connection, groups[0].id)[0]
    assert_valid_domain_scopes(plugin, scope)
    scope_config = scope_config or plugin.scope_config_type()
    assert_valid_pipeline_plan(plugin, connection, scope, scope_config)
    for stream in plugin.streams:
        if isinstance(stream, type):
            stream = stream(plugin.name)
        assert_stream_run(stream, connection, scope, scope_config)
