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

from datetime import datetime, timezone
from typing import Optional
from zoneinfo import ZoneInfo, available_timezones


def validate_timezone_name(timezone_name: str) -> str:
    timezone_name = timezone_name.strip()
    if not timezone_name:
        raise ValueError('timezone is required')
    if timezone_name not in available_timezones():
        raise ValueError(f'unknown timezone: {timezone_name}')
    return timezone_name


def connection_timezone(timezone_name: str) -> ZoneInfo:
    return ZoneInfo(validate_timezone_name(timezone_name))


def utc_now() -> datetime:
    return datetime.now(timezone.utc).replace(tzinfo=None)


def to_utc(dt: Optional[datetime], tz: ZoneInfo) -> Optional[datetime]:
    if dt is None:
        return None
    if dt.tzinfo is not None:
        return dt.astimezone(timezone.utc).replace(tzinfo=None)
    return dt.replace(tzinfo=tz).astimezone(timezone.utc).replace(tzinfo=None)


def glpi_search_time_from_iso(iso_time: str, tz: ZoneInfo) -> str:
    normalized = iso_time.replace('Z', '+00:00')
    dt = datetime.fromisoformat(normalized)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(tz).strftime('%Y-%m-%d %H:%M:%S')
