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

from glpi.parsing import on_demand_id_from_url, parse_glpi_dropdown, parse_glpi_user


def test_parse_glpi_dropdown_with_name_and_id():
    name, user_id = parse_glpi_dropdown('Jane Smith (103)')
    assert name == 'Jane Smith'
    assert user_id == 103


def test_parse_glpi_dropdown_with_numeric_id():
    name, user_id = parse_glpi_dropdown(101)
    assert name is None
    assert user_id == 101


def test_parse_glpi_user_with_id():
    name, user_id = parse_glpi_user('John Doe (101)')
    assert name == 'John Doe'
    assert user_id == '101'


def test_parse_glpi_user_without_id():
    name, user_id = parse_glpi_user('John Doe')
    assert name == 'John Doe'
    assert user_id is None


def test_on_demand_id_from_url_uses_raw_id():
    assert on_demand_id_from_url({'id': 7}, 'User') == 7


def test_on_demand_id_from_url_parses_request_url():
    raw = {'x_request_url': 'http://glpi.example.com/apirest.php/User/7'}
    assert on_demand_id_from_url(raw, 'User') == 7
