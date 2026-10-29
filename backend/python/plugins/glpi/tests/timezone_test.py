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

from glpi.timezone import connection_timezone, glpi_search_time_from_iso


def test_glpi_search_time_from_iso_converts_to_connection_timezone():
    tz = connection_timezone('Europe/Paris')
    assert glpi_search_time_from_iso('2024-06-01T00:00:00Z', tz) == '2024-06-01 02:00:00'
