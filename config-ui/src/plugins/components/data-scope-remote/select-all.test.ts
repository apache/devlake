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

import { describe, it, expect, vi } from 'vitest';

import { fetchAllSearchResults, getSelectAllState, toggleAllFiltered } from './select-all';

const items = (from: number, to: number) => Array.from({ length: to - from }, (_, i) => ({ id: `p${from + i}` }));

describe('fetchAllSearchResults', () => {
  it('loads pages until a short page is returned', async () => {
    const fetchPage = vi
      .fn()
      .mockResolvedValueOnce({ children: items(0, 2) })
      .mockResolvedValueOnce({ children: items(2, 4) })
      .mockResolvedValueOnce({ children: items(4, 5) });

    const res = await fetchAllSearchResults(fetchPage, 2);

    expect(res.items.map((it) => it.id)).toEqual(['p0', 'p1', 'p2', 'p3', 'p4']);
    expect(res.complete).toBe(true);
    expect(res.total).toBeUndefined();
    expect(fetchPage).toHaveBeenCalledTimes(3);
    expect(fetchPage).toHaveBeenNthCalledWith(3, 3, 2);
  });

  it('stops when the reported total is reached', async () => {
    const fetchPage = vi.fn().mockResolvedValue({ children: items(0, 2), count: 2 });
    const res = await fetchAllSearchResults(fetchPage, 2);
    expect(fetchPage).toHaveBeenCalledTimes(1);
    expect(res).toMatchObject({ total: 2, complete: true });
  });

  it('uses the reported total to avoid an extra request for the last full page', async () => {
    const fetchPage = vi
      .fn()
      .mockResolvedValueOnce({ children: items(0, 2), count: 4 })
      .mockResolvedValueOnce({ children: items(2, 4), count: 4 });
    const res = await fetchAllSearchResults(fetchPage, 2);
    expect(fetchPage).toHaveBeenCalledTimes(2);
    expect(res.items).toHaveLength(4);
  });

  it('respects the max page limit and removes duplicates', async () => {
    const fetchPage = vi.fn().mockResolvedValue({ children: items(0, 2) });
    const res = await fetchAllSearchResults(fetchPage, 2, 3);
    expect(fetchPage).toHaveBeenCalledTimes(3);
    expect(res.items).toHaveLength(2);
    expect(res.complete).toBe(false);
  });

  it('handles empty responses', async () => {
    const fetchPage = vi.fn().mockResolvedValue({ children: null });
    expect(await fetchAllSearchResults(fetchPage, 2)).toEqual({ items: [], total: undefined, complete: true });
  });
});

describe('toggleAllFiltered / getSelectAllState', () => {
  const filtered = items(0, 3);

  it('adds all filtered items and keeps existing selection', () => {
    const selected = [{ id: 'other' }, { id: 'p1' }];
    const res = toggleAllFiltered(selected, filtered);
    expect(res.map((it) => it.id)).toEqual(['other', 'p1', 'p0', 'p2']);
    expect(getSelectAllState(selected, filtered)).toBe('some');
    expect(getSelectAllState(res, filtered)).toBe('all');
  });

  it('removes all filtered items when all are already selected', () => {
    const selected = [{ id: 'other' }, ...filtered];
    const res = toggleAllFiltered(selected, filtered);
    expect(res.map((it) => it.id)).toEqual(['other']);
    expect(getSelectAllState(res, filtered)).toBe('none');
  });

  it('ignores disabled items', () => {
    const res = toggleAllFiltered([], filtered, ['p1']);
    expect(res.map((it) => it.id)).toEqual(['p0', 'p2']);
    expect(getSelectAllState(res, filtered, ['p1'])).toBe('all');
    expect(toggleAllFiltered(res, filtered, ['p1'])).toEqual([]);
  });

  it('compares ids independent of their type', () => {
    expect(getSelectAllState([{ id: 1 }], [{ id: '1' }])).toBe('all');
  });
});
