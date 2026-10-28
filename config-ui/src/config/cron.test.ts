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

import { cronPresets, getCron, getCronOptions } from './cron';

describe('cronPresets', () => {
  it('includes Daily, Weekly, and Monthly', () => {
    expect(cronPresets).toHaveLength(3);
    expect(cronPresets.map((p) => p.label)).toEqual(['Daily', 'Weekly', 'Monthly']);
  });

  it('has valid cron expressions', () => {
    expect(cronPresets[0].config).toBe('0 0 * * *');
    expect(cronPresets[1].config).toBe('0 0 * * 1');
    expect(cronPresets[2].config).toBe('0 0 1 * *');
  });
});

describe('getCron', () => {
  it('returns manual config when isManual is true', () => {
    const result = getCron(true, '');
    expect(result).toEqual({
      label: 'Manual',
      config: '',
      description: '',
      nextTime: '',
      nextTimes: [],
    });
  });

  it('matches a known preset and computes next times', () => {
    const result = getCron(false, '0 0 * * *');
    expect(result.label).toBe('Daily');
    expect(result.description).toBe('(at 00:00AM UTC)');
    expect(result.nextTime).toBeTruthy();
    expect(result.nextTimes).toHaveLength(3);
  });

  it('returns Custom label for a non-preset cron expression', () => {
    const result = getCron(false, '0 12 * * *');
    expect(result.label).toBe('Custom');
    expect(result.config).toBe('0 12 * * *');
    expect(result.nextTime).toBeTruthy();
    expect(result.nextTimes).toHaveLength(3);
  });

  it('handles an invalid cron expression gracefully', () => {
    const result = getCron(false, 'invalid');
    expect(result.label).toBe('Custom');
    expect(result.nextTime).toBeNull();
    expect(result.nextTimes).toEqual([]);
  });
});

describe('getCronOptions', () => {
  it('returns Manual + presets + Custom', () => {
    const options = getCronOptions();
    expect(options).toHaveLength(5);
    expect(options[0]).toEqual({ label: 'Manual', value: 'manual', subLabel: '' });
    expect(options[4]).toEqual({ label: 'Custom', value: 'custom', subLabel: '' });
  });

  it('includes preset values in the middle', () => {
    const options = getCronOptions();
    expect(options[1].value).toBe('0 0 * * *');
    expect(options[2].value).toBe('0 0 * * 1');
    expect(options[3].value).toBe('0 0 1 * *');
  });
});
