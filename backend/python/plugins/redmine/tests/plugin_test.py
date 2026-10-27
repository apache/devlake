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

from redmine.main import RedminePlugin
from redmine.models import Connection, ScopeConfig


def test_valid_plugin():
    assert_valid_plugin(RedminePlugin())


def test_valid_plugin_and_connection():
    endpoint = os.environ.get('REDMINE_ENDPOINT')
    token = os.environ.get('REDMINE_TOKEN')
    if not (endpoint and token):
        pytest.skip("REDMINE_ENDPOINT and REDMINE_TOKEN env vars required")

    connection = Connection(id=1, name='test_connection', endpoint=endpoint, token=token)
    scope_config = ScopeConfig(id=1, name='test_config')
    assert_plugin_run(RedminePlugin(), connection, scope_config)
