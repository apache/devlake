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

import { validRawPlan } from './utils';

describe('validRawPlan', () => {
  it('returns true for invalid JSON', () => {
    expect(validRawPlan('not json')).toBe(true);
    expect(validRawPlan('{broken')).toBe(true);
  });

  it('returns true for empty nested arrays', () => {
    expect(validRawPlan('[[]]')).toBe(true);
    expect(validRawPlan('[]')).toBe(true);
  });

  it('returns false when plan contains tasks', () => {
    expect(validRawPlan('[["task1"]]')).toBe(false);
    expect(validRawPlan('[[{"plugin":"github"}]]')).toBe(false);
  });

  it('returns false for a flat array with items', () => {
    expect(validRawPlan('["item"]')).toBe(false);
  });
});
