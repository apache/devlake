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
from functools import wraps
from typing import Generator, TextIO, Optional

from urllib.parse import urlparse, parse_qsl
from fire.decorators import SetParseFn
from sqlmodel import create_engine
from sqlalchemy.engine import Engine

from pydevlake.collection_time import parse_time_after
from pydevlake.context import Context
from pydevlake.message import Message
from pydevlake.model import SubtaskRun
from pydevlake.config import set_config


def plugin_method(func):

    def open_send_channel() -> TextIO:
        fd = 3
        return os.fdopen(fd, 'w')

    def send_output(send_ch: TextIO, obj: object):
        if not isinstance(obj, Message):
            raise Exception(f"Not a message: {obj}")
        send_ch.write(obj.json(exclude_none=True, by_alias=True))
        send_ch.write('\n')
        send_ch.flush()

    def parse_arg(arg):
        try:
            return json.loads(arg)
        except json.JSONDecodeError as e:
            raise Exception(f"Invalid JSON {arg}: {e.msg}")

    @wraps(func)
    @SetParseFn(parse_arg)
    def wrapper(self, *args):
        # first arg will always be arg - pluck it out
        cfg = args[0]
        set_config(cfg)
        ret = func(self, *args)
        if ret is not None:
            with open_send_channel() as send_ch:
                if isinstance(ret, Generator):
                    for each in ret:
                        send_output(send_ch, each)
                else:
                    send_output(send_ch, ret)
        return None

    return wrapper


# all remote-callable (@plugin_method) methods should have a _ placeholder first param (until/unless I figure out a better way)
class PluginCommands:
    def __init__(self, plugin):
        self._plugin = plugin

    @plugin_method
    def collect(self, _, ctx: dict, stream: str):
        yield from self._plugin.collect(self._mk_context(ctx), stream)

    @plugin_method
    def extract(self, _, ctx: dict, stream: str):
        yield from self._plugin.extract(self._mk_context(ctx), stream)

    @plugin_method
    def convert(self, _, ctx: dict, stream: str):
        yield from self._plugin.convert(self._mk_context(ctx), stream)

    @plugin_method
    def test_connection(self, _, connection: dict):
        if "name" not in connection:
            connection["name"] = "Test connection"
        connection = self._plugin.connection_type(**connection)
        return self._plugin.test_connection(connection)

    @plugin_method
    def make_pipeline(self, _, scope_config_pairs: list[tuple[dict, dict]], connection: dict):
        connection = self._plugin.connection_type(**connection)
        scope_config_pairs = [
            (
                self._plugin.tool_scope_type(**raw_scope),
                self._plugin.scope_config_type(**raw_config or {})
            )
            for raw_scope, raw_config in scope_config_pairs
        ]
        return self._plugin.make_pipeline(scope_config_pairs, connection)

    @plugin_method
    def plugin_info(self, _):
        return self._plugin.plugin_info()

    @plugin_method
    def remote_scopes(self, _, connection: dict, group_id: Optional[str] = None):
        c = self._plugin.connection_type(**connection)
        yield from self._plugin.make_remote_scopes(c, group_id)

    def _mk_context(self, data: dict):
        db_url = data['db_url']
        scope_dict = data['scope']
        scope = self._plugin.tool_scope_type(**scope_dict)
        connection_dict = data['connection']
        connection = self._plugin.connection_type(**connection_dict)
        scope_config_dict = data.get('scope_config') or {}
        scope_config = self._plugin.scope_config_type(**scope_config_dict)
        options = data.get('options', {})
        sync_policy = data.get('sync_policy') or {}
        time_after = parse_time_after(sync_policy.get('timeAfter'))
        return Context(create_db_engine(db_url), connection, scope, scope_config, options, time_after)

def create_db_engine(db_url) -> Engine:
    # SQLAlchemy doesn't understand postgres:// scheme
    db_url = db_url.replace("postgres://", "postgresql://")
    # Remove query args
    base_url = db_url.split('?')[0]
    # `parseTime` parameter is not understood by MySQL driver,
    # so we have to parse query args to remove it
    connect_args = dict(parse_qsl(urlparse(db_url).query))
    if 'parseTime' in connect_args:
        del connect_args['parseTime']
    if 'loc' in connect_args:
        del connect_args['loc']
    if 'tls' in connect_args:
        del connect_args['tls']
        connect_args['ssl'] = {'verify_cert': 'False'}
    try:
        engine = create_engine(base_url, connect_args=connect_args)
        tables = SubtaskRun.metadata.tables
        tables[SubtaskRun.__tablename__].create(engine, checkfirst=True)
        return engine
    except Exception as e:
        raise Exception(f"Unable to make a database connection") from e
