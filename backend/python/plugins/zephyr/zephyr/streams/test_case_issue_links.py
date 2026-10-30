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

from typing import Iterable

from zephyr.models import TestCase, TestCaseIssueLink
from zephyr.streams.test_cases import TestCases
from zephyr.utils import parse_json_list
from pydevlake import Substream, DomainType


class TestCaseIssueLinks(Substream):
    tool_model = TestCaseIssueLink
    domain_types = [DomainType.CODE_QUALITY]
    domain_models = []
    parent_stream = TestCases

    def collect(self, state, context, parent: TestCase) -> Iterable[tuple[object, dict]]:
        for link in parse_json_list(parent.issue_links_json):
            link_id = link.get('id')
            if link_id is None:
                continue
            yield {
                'id': link_id,
                'test_case_id': parent.id,
                'test_case_key': parent.key,
                'issue_id': link.get('issueId'),
                'link_type': link.get('type'),
            }, state

    def extract(self, raw: dict) -> TestCaseIssueLink:
        return TestCaseIssueLink(
            id=raw['id'],
            test_case_id=raw.get('test_case_id'),
            test_case_key=raw.get('test_case_key'),
            issue_id=raw.get('issue_id'),
            link_type=raw.get('link_type'),
        )

    def convert(self, link: TestCaseIssueLink, ctx):
        yield from ()
