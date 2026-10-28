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

import { describe, it, expect } from 'vitest';

import { formatTime } from './time';

describe('formatTime', () => {
  it('formats a date string with the default YYYY-MM-DD HH:mm pattern', () => {
    const result = formatTime('2024-06-15T12:00:00Z');
    expect(result).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/);
  });

  it('formats with a custom format', () => {
    const result = formatTime('2024-06-15T12:00:00Z', 'YYYY');
    expect(result).toBe('2024');
  });

  it('formats a Date object', () => {
    const result = formatTime(new Date('2024-01-01T00:00:00Z'), 'YYYY');
    expect(result).toBe('2024');
  });

  it('returns dash for null', () => {
    expect(formatTime(null)).toBe('-');
  });

  it('returns dash for empty string', () => {
    expect(formatTime('')).toBe('-');
  });
});
