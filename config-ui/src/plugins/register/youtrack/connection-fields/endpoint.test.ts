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

import { validateEndpoint } from './endpoint';

describe('validateEndpoint', () => {
  it('accepts the InCloud shape with the /youtrack context path', () => {
    expect(validateEndpoint('https://example.myjetbrains.com/youtrack/api')).toBe('');
  });

  it('accepts the youtrack.cloud shape', () => {
    expect(validateEndpoint('https://example.youtrack.cloud/api')).toBe('');
  });

  it('accepts self-hosted shapes, with and without a context path', () => {
    expect(validateEndpoint('https://youtrack.example.com/api')).toBe('');
    expect(validateEndpoint('https://www.example.com/youtrack/api')).toBe('');
  });

  it('accepts http for self-hosted', () => {
    expect(validateEndpoint('http://youtrack.internal:8080/api')).toBe('');
  });

  it('accepts a trailing slash (normalized like the backend)', () => {
    expect(validateEndpoint('https://example.myjetbrains.com/youtrack/api/')).toBe('');
    expect(validateEndpoint('https://example.myjetbrains.com/youtrack/api//')).toBe('');
  });

  it('rejects an empty endpoint as required', () => {
    expect(validateEndpoint('')).toBe('endpoint is required');
  });

  it('rejects a URL that does not end in /api, with the example in the message', () => {
    const message = validateEndpoint('https://example.myjetbrains.com/youtrack');
    expect(message).toContain('/api');
    expect(message).toContain('https://example.myjetbrains.com/youtrack/api');
  });

  it('rejects the most common InCloud error — the missing /youtrack prefix surfaces as a non-/api path', () => {
    expect(validateEndpoint('https://example.myjetbrains.com')).not.toBe('');
  });

  it('rejects non-absolute URLs', () => {
    expect(validateEndpoint('example.myjetbrains.com/youtrack/api')).not.toBe('');
    expect(validateEndpoint('/youtrack/api')).not.toBe('');
  });

  it('rejects non-http(s) schemes', () => {
    expect(validateEndpoint('ftp://example.com/api')).not.toBe('');
  });
});
