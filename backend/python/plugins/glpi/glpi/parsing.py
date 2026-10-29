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

import re
from typing import Optional, Union

from glpi.models import User
from pydevlake.model import domain_id

# GLPI may return dropdown/user fields as bare ids or as "Display Name (123)"
# when expand_dropdowns=true; split that combined label into name and numeric id.
_USER_SUFFIX_RE = re.compile(r'^(.*?) \((\d+)\)$')


def on_demand_id_from_url(raw: dict, resource: str) -> Optional[int]:
    item_id = raw.get('id')
    if item_id is not None:
        return int(item_id)
    match = re.search(rf'/{resource}/(\d+)(?:/|$)', raw.get('x_request_url', ''))
    return int(match.group(1)) if match else None


def parse_glpi_user(value: Optional[str]) -> tuple[Optional[str], Optional[str]]:
    if not value:
        return None, None
    value = str(value).strip()
    if not value or value == '\xa0':
        return None, None
    match = _USER_SUFFIX_RE.match(value)
    return (match.group(1).strip(), match.group(2)) if match else (value, None)


def parse_glpi_dropdown(value: Optional[Union[str, int]]) -> tuple[Optional[str], Optional[int]]:
    if value is None:
        return None, None
    if isinstance(value, int):
        return None, value
    text = str(value).strip()
    if not text or text == '\xa0':
        return None, None
    name, id_str = parse_glpi_user(text)
    if id_str:
        return name, int(id_str)
    if text.isdigit():
        return None, int(text)
    return text, None


def user_account_id(connection_id: int, user_id: Optional[Union[int, str]]) -> Optional[str]:
    return domain_id(User, connection_id, int(user_id)) if user_id is not None else None
