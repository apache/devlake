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

from zephyr.models import TestCase, TestCaseLabel
from zephyr.streams.test_cases import TestCases
from zephyr.utils import parse_json_list
from pydevlake import Substream, DomainType


class TestCaseLabels(Substream):
    tool_model = TestCaseLabel
    domain_types = [DomainType.CODE_QUALITY]
    domain_models = []
    parent_stream = TestCases

    def collect(self, state, context, parent: TestCase) -> Iterable[tuple[object, dict]]:
        for label in parse_json_list(parent.labels_json):
            yield {
                'id': f'{parent.id}:{label}',
                'test_case_id': parent.id,
                'test_case_key': parent.key,
                'label_name': label,
            }, state

    def extract(self, raw: dict) -> TestCaseLabel:
        return TestCaseLabel(
            id=raw['id'],
            test_case_id=raw.get('test_case_id'),
            test_case_key=raw.get('test_case_key'),
            label_name=raw.get('label_name'),
        )

    def convert(self, label: TestCaseLabel, ctx):
        yield from ()
