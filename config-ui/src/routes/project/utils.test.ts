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

import { validName, encodeName } from './utils';

describe('validName', () => {
  it('accepts alphanumeric names with hyphens and underscores', () => {
    expect(validName('my-project')).toBe(true);
    expect(validName('my_project')).toBe(true);
    expect(validName('myProject123')).toBe(true);
  });

  it('accepts names with forward slashes', () => {
    expect(validName('org/project')).toBe(true);
    expect(validName('a/b/c')).toBe(true);
  });

  it('rejects empty string', () => {
    expect(validName('')).toBe(false);
  });

  it('rejects names with spaces', () => {
    expect(validName('my project')).toBe(false);
  });

  it('rejects names with special characters', () => {
    expect(validName('my@project')).toBe(false);
    expect(validName('hello!')).toBe(false);
    expect(validName('a b')).toBe(false);
  });
});

describe('encodeName', () => {
  it('encodes forward slashes', () => {
    expect(encodeName('org/project')).toBe('org%2Fproject');
  });

  it('encodes spaces', () => {
    expect(encodeName('hello world')).toBe('hello%20world');
  });

  it('passes through simple alphanumeric strings', () => {
    expect(encodeName('simple')).toBe('simple');
  });
});
