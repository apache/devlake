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

from zephyr.utils import extract_issue_links, parse_json_list, parse_json_object


def test_parse_json_list_handles_empty_invalid_and_non_list():
    assert parse_json_list(None) == []
    assert parse_json_list('') == []
    assert parse_json_list('not-json') == []
    assert parse_json_list('{"a": 1}') == []
    assert parse_json_list('["a", "b"]') == ['a', 'b']


def test_parse_json_object_handles_empty_invalid_and_non_object():
    assert parse_json_object(None) is None
    assert parse_json_object('') is None
    assert parse_json_object('not-json') is None
    assert parse_json_object('["a"]') is None
    assert parse_json_object('{"a": 1}') == {'a': 1}


def test_extract_issue_links_reads_issues_array():
    assert extract_issue_links(None) == []
    assert extract_issue_links({}) == []
    assert extract_issue_links({'issues': 'bad'}) == []
    assert extract_issue_links({'issues': [{'id': 1}]}) == [{'id': 1}]
