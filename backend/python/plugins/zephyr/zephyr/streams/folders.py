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

from zephyr.api import ZephyrAPI
from zephyr.models import Folder, Project
from zephyr.utils import attach_x_request, scope_project_key
from pydevlake import Stream, DomainType


class Folders(Stream):
    tool_model = Folder
    domain_types = [DomainType.CODE_QUALITY]
    domain_models = []

    def collect(self, state, context) -> Iterable[tuple[object, dict]]:
        project: Project = context.scope
        project_key = scope_project_key(project)
        with ZephyrAPI(context.connection) as api:
            for raw in api.folders(project_key):
                raw['projectKey'] = project_key
                attach_x_request(raw, f'{api.base_url}/folders', projectKey=project_key)
                yield raw, state

    def extract(self, raw: dict) -> Folder:
        return Folder(
            id=raw['id'],
            project_key=raw.get('projectKey'),
            name=raw.get('name'),
            parent_id=raw.get('parentId'),
            folder_type=raw.get('folderType'),
            index=raw.get('index'),
        )

    def convert(self, folder: Folder, ctx):
        yield from ()
