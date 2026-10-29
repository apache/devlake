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

from contextlib import contextmanager
from types import ModuleType
from typing import Iterator
from unittest.mock import create_autospec, patch

from pydevlake.model import RawModel
from pydevlake.stream import Stream
from pydevlake.testing import ContextBuilder
from sqlmodel import Session

from glpi.api import GlpiAPI
from glpi.main import GlpiPlugin
from glpi.streams import tickets as tickets_stream

GLPI_API_BASE_URL = 'http://glpi.example.com'


def ensure_stream_raw_table(engine, stream: Stream) -> None:
    table_name = stream.raw_model_table
    if table_name not in RawModel.metadata.tables:
        with Session(engine) as session:
            stream.raw_model(session)
        return
    RawModel.metadata.tables[table_name].create(engine, checkfirst=True)


def collect_context(
    scope_id='1',
    *,
    scope_name='Test Entity',
    scope_completename=None,
    **scope_config_kwargs,
):
    scope_kwargs = {'name': scope_name}
    if scope_completename is not None:
        scope_kwargs['completename'] = scope_completename
    return (
        ContextBuilder(GlpiPlugin())
        .with_connection(
            endpoint=GLPI_API_BASE_URL,
            token='test_app_token',
            secret_key='test_user_token',
            timezone='UTC',
        )
        .with_scope(scope_id, **scope_kwargs)
        .with_scope_config(**scope_config_kwargs)
        .build()
    )


@contextmanager
def glpi_api_mock(module: ModuleType = tickets_stream) -> Iterator[GlpiAPI]:
    api = create_autospec(GlpiAPI, instance=True)
    api.base_url = GLPI_API_BASE_URL
    with patch.object(module, 'GlpiAPI') as mock_cls:
        mock_cls.return_value.__enter__.return_value = api
        yield api
