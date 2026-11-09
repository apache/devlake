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

from typing import Optional

import pytest

pytest.register_assert_rewrite('pydevlake.testing')

from sqlmodel import Field as _Field


def Field(*args, primary_key: bool = False, auto_increment: Optional[bool] = None, source: Optional[str] = None,
          **kwargs):
    """
    A wrapper around sqlmodel.Field that adds a source parameter.
    """
    schema_extra = kwargs.pop('schema_extra', {})
    if source is not None:
        schema_extra['source'] = source
    if primary_key:
        schema_extra['primaryKey'] = True
    if auto_increment is not None:
        schema_extra['autoIncrement'] = auto_increment
    # Only pass primary_key if True; SQLModel 0.0.38 rejects it alongside sa_column
    if primary_key:
        field = _Field(*args, **kwargs, primary_key=True, schema_extra=schema_extra)
    elif schema_extra:
        field = _Field(*args, **kwargs, schema_extra=schema_extra)
    else:
        field = _Field(*args, **kwargs)
    # SQLModel 0.0.38 doesn't propagate schema_extra to FieldInfo.json_schema_extra,
    # so we set it manually for consumers like autoextract and model_json_schema.
    if schema_extra:
        field.json_schema_extra = schema_extra
    return field



from .model import ToolModel, ToolScope, DomainScope, Connection, ScopeConfig, DomainType, domain_id
from .logger import logger
from .message import RemoteScopeGroup, TestConnectionResult
from .plugin import Plugin, ScopeConfigPair
from .stream import Stream, Substream
from .context import Context

# the debugger hangs on startup during plugin registration (reason unknown), hence this workaround
import sys

if not sys.argv.__contains__('startup'):
    from pydevlake.helpers import debugger

    debugger.init()
