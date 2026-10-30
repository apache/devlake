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
import json
import re
from typing import Any, Optional

from zephyr.api import ProjectLookups

_RESOURCE_KEY_RE = re.compile(r'/([^/]+)$')


def parse_datetime(value) -> Optional[datetime.datetime]:
    if value is None:
        return None
    elif isinstance(value, datetime.datetime):
        return value
    elif isinstance(value, str) and value:
        return datetime.datetime.fromisoformat(value.replace('Z', '+00:00'))
    else:
        return None


def _ensure_utc(value: datetime.datetime) -> datetime.datetime:
    if value.tzinfo is None:
        return value.replace(tzinfo=datetime.timezone.utc)
    return value.astimezone(datetime.timezone.utc)


def is_on_or_after(value, cutoff: datetime.datetime) -> bool:
    parsed = parse_datetime(value)
    if parsed is None:
        return True
    return _ensure_utc(parsed) >= _ensure_utc(cutoff)


def passes_time_filter(value, cutoff: Optional[datetime.datetime]) -> bool:
    return cutoff is None or is_on_or_after(value, cutoff)


def scope_project_key(project) -> str:
    return project.project_key or project.id


def attach_x_request(raw: dict, url: str, **input) -> None:
    raw['x_request_url'] = url
    raw['x_request_input'] = input


def resource_id(ref: Optional[dict]) -> Optional[int]:
    if not ref:
        return None
    ref_id = ref.get('id')
    return int(ref_id) if ref_id is not None else None


def resource_key(ref: Optional[dict]) -> Optional[str]:
    if not ref:
        return None
    if ref.get('key'):
        return ref['key']
    self_url = ref.get('self') or ''
    match = _RESOURCE_KEY_RE.search(self_url)
    return match.group(1) if match else None


def lookup_name(lookups: Optional[ProjectLookups], table: str, ref: Optional[dict]) -> Optional[str]:
    ref_id = resource_id(ref)
    if ref_id is None:
        return ref.get('name') if ref else None
    if not lookups:
        return ref.get('name') if ref else None
    bucket = getattr(lookups, table, {})
    entry = bucket.get(ref_id)
    if entry:
        return entry.get('name')
    return ref.get('name') if ref else None


def json_dumps(value: Any) -> Optional[str]:
    return json.dumps(value) if value is not None else None


def parse_json_list(value: Optional[str]) -> list:
    if not value:
        return []
    try:
        parsed = json.loads(value)
        return parsed if isinstance(parsed, list) else []
    except json.JSONDecodeError:
        return []


def parse_json_object(value: Optional[str]) -> Optional[dict]:
    if not value:
        return None
    try:
        parsed = json.loads(value)
        return parsed if isinstance(parsed, dict) else None
    except json.JSONDecodeError:
        return None


def extract_issue_links(links: Optional[dict]) -> list[dict]:
    if not links:
        return []
    issues = links.get('issues') or []
    return issues if isinstance(issues, list) else []
