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

import { EntitiesLabel, transformEntities } from './entities';

describe('EntitiesLabel', () => {
  it('maps all known domain entity types', () => {
    expect(EntitiesLabel.CODE).toBe('Source Code Management');
    expect(EntitiesLabel.TICKET).toBe('Issue Tracking');
    expect(EntitiesLabel.CODEREVIEW).toBe('Code Review');
    expect(EntitiesLabel.CICD).toBe('CI/CD');
    expect(EntitiesLabel.CROSS).toBe('Cross Domain');
    expect(EntitiesLabel.CLAUDE_CODE).toBe('Claude Code');
    expect(EntitiesLabel.CODEQUALITY).toBe('Code Quality Domain');
  });

  it('contains exactly 7 entity types', () => {
    expect(Object.keys(EntitiesLabel)).toHaveLength(7);
  });
});

describe('transformEntities', () => {
  it('transforms entity keys to label-value pairs', () => {
    const result = transformEntities(['CODE', 'TICKET']);
    expect(result).toEqual([
      { label: 'Source Code Management', value: 'CODE' },
      { label: 'Issue Tracking', value: 'TICKET' },
    ]);
  });

  it('returns undefined label for unknown entity', () => {
    const result = transformEntities(['UNKNOWN']);
    expect(result).toEqual([{ label: undefined, value: 'UNKNOWN' }]);
  });

  it('handles empty array', () => {
    expect(transformEntities([])).toEqual([]);
  });

  it('preserves order', () => {
    const result = transformEntities(['CICD', 'CODE', 'CROSS']);
    expect(result.map((r) => r.value)).toEqual(['CICD', 'CODE', 'CROSS']);
  });
});
