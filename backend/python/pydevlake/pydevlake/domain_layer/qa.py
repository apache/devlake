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

from datetime import datetime
from typing import Optional

from sqlmodel import Field

from pydevlake.model import DomainModel, DomainScope


class QaProject(DomainScope, table=True):
    __tablename__ = 'qa_projects'
    name: str


class QaTestCase(DomainModel, table=True):
    __tablename__ = 'qa_test_cases'
    name: str
    create_time: Optional[datetime]
    creator_id: Optional[str]
    type: Optional[str]
    qa_api_id: Optional[str]
    qa_project_id: str


class QaTestCaseExecution(DomainModel, table=True):
    __tablename__ = 'qa_test_case_executions'
    qa_project_id: str
    qa_test_case_id: str
    create_time: Optional[datetime]
    start_time: Optional[datetime]
    finish_time: Optional[datetime]
    creator_id: Optional[str]
    status: Optional[str]
    is_invalid: bool = False
