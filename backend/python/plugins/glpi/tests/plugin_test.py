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
import pytest

from pydevlake.testing import assert_valid_plugin, assert_plugin_run

from glpi.main import GlpiPlugin
from glpi.models import Connection, ScopeConfig


def test_valid_plugin():
    assert_valid_plugin(GlpiPlugin())


def test_valid_plugin_and_connection():
    endpoint = os.environ.get('GLPI_ENDPOINT')
    token = os.environ.get('GLPI_TOKEN')
    secret_key = os.environ.get('GLPI_SECRET_KEY')
    if not (endpoint and token and secret_key):
        pytest.skip("GLPI_ENDPOINT, GLPI_TOKEN, and GLPI_SECRET_KEY env vars required")

    connection = Connection(
        id=1,
        name='test_connection',
        endpoint=endpoint,
        token=token,
        secret_key=secret_key,
        timezone='UTC',
    )
    scope_config = ScopeConfig(id=1, name='test_config')
    assert_plugin_run(GlpiPlugin(), connection, scope_config)
