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
from typing import Optional

from sqlalchemy.engine import Engine

from pydevlake.model import Connection, ScopeConfig, ToolScope


class Context:
    def __init__(self,
                 engine: Engine,
                 connection: Connection,
                 scope: ToolScope,
                 scope_config: ScopeConfig,
                 options: dict = None,
                 time_after: Optional[datetime.datetime] = None):
        self.engine = engine
        self.connection = connection
        self.scope = scope
        self.scope_config = scope_config
        self.options = options or {}
        self.time_after = time_after
        self._engine = None

    @property
    def incremental(self) -> bool:
        return self.options.get('incremental') is True
