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

from pydevlake.collection_time import collection_time_after, parse_time_after
from pydevlake.context import Context


def test_collection_time_after_uses_sync_policy():
    context = Context(
        engine=None,
        connection=None,
        scope=None,
        scope_config=None,
        time_after=datetime.datetime(2024, 1, 1, tzinfo=datetime.timezone.utc),
    )
    assert collection_time_after(context) == datetime.datetime(2024, 1, 1, tzinfo=datetime.timezone.utc)


def test_collection_time_after_skipped_when_incremental():
    context = Context(
        engine=None,
        connection=None,
        scope=None,
        scope_config=None,
        options={'incremental': True},
        time_after=datetime.datetime(2024, 1, 1, tzinfo=datetime.timezone.utc),
    )
    assert collection_time_after(context) is None


def test_parse_time_after_accepts_rfc3339():
    parsed = parse_time_after('2024-06-01T00:00:00Z')
    assert parsed == datetime.datetime(2024, 6, 1, tzinfo=datetime.timezone.utc)
