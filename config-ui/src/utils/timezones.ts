/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

export interface TimezoneOption {
  value: string;
  label: string;
}

let cachedOptions: TimezoneOption[] | null = null;

export function getBrowserTimezone(): string | undefined {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch {
    return undefined;
  }
}

function getUtcOffsetMinutes(timezone: string, date = new Date()): number {
  try {
    const utcDate = new Date(date.toLocaleString('en-US', { timeZone: 'UTC' }));
    const tzDate = new Date(date.toLocaleString('en-US', { timeZone: timezone }));
    return (tzDate.getTime() - utcDate.getTime()) / 60_000;
  } catch {
    return 0;
  }
}

function sortTimezones(timezones: string[]): string[] {
  return [...timezones].sort((left, right) => {
    const offsetDiff = getUtcOffsetMinutes(left) - getUtcOffsetMinutes(right);
    if (offsetDiff !== 0) {
      return offsetDiff;
    }
    return left.localeCompare(right);
  });
}

function listIanaTimezones(): string[] {
  if ('supportedValuesOf' in Intl && typeof Intl.supportedValuesOf === 'function') {
    return sortTimezones(Intl.supportedValuesOf('timeZone'));
  }

  const fallback = new Set(['UTC']);
  const browserTimezone = getBrowserTimezone();
  if (browserTimezone) {
    fallback.add(browserTimezone);
  }
  return sortTimezones([...fallback]);
}

export function formatTimezoneLabel(timezone: string): string {
  try {
    const parts = new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      timeZoneName: 'shortOffset',
    }).formatToParts(new Date());
    const offset = parts.find((part) => part.type === 'timeZoneName')?.value ?? '';
    const name = timezone.replace(/_/g, ' ');
    return offset ? `${name} (${offset})` : name;
  } catch {
    return timezone.replace(/_/g, ' ');
  }
}

export function getTimezoneOptions(): TimezoneOption[] {
  if (!cachedOptions) {
    cachedOptions = listIanaTimezones().map((timezone) => ({
      value: timezone,
      label: formatTimezoneLabel(timezone),
    }));
  }
  return cachedOptions;
}

export function buildTimezoneOptions(currentValue?: string): TimezoneOption[] {
  const options = getTimezoneOptions();
  if (currentValue && !options.some((option) => option.value === currentValue)) {
    return sortTimezones([currentValue, ...options.map((option) => option.value)]).map((timezone) => ({
      value: timezone,
      label: formatTimezoneLabel(timezone),
    }));
  }
  return options;
}
