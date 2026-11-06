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

export const SELECT_ALL_PAGE_SIZE = 100;
export const SELECT_ALL_MAX_PAGES = 100;

type WithId = { id: ID };

export type FetchAllResult<T> = {
  items: T[];
  /** total reported by the backend (`count`), undefined if the plugin doesn't provide it */
  total?: number;
  /** false if loading stopped because of the page limit, i.e. there may be more results */
  complete: boolean;
};

/**
 * Loads every page of a remote search. Stops as soon as the backend-reported total (`count`) is
 * reached or – for plugins that don't report a total – a page returns fewer items than requested.
 */
export const fetchAllSearchResults = async <T extends WithId>(
  fetchPage: (page: number, pageSize: number) => Promise<{ children?: T[] | null; count?: number }>,
  pageSize = SELECT_ALL_PAGE_SIZE,
  maxPages = SELECT_ALL_MAX_PAGES,
): Promise<FetchAllResult<T>> => {
  const items: T[] = [];
  const seen = new Set<string>();
  let total: number | undefined;

  for (let page = 1; page <= maxPages; page++) {
    const res = await fetchPage(page, pageSize);
    const children = res?.children ?? [];

    children.forEach((it) => {
      const key = String(it.id);
      if (!seen.has(key)) {
        seen.add(key);
        items.push(it);
      }
    });

    if (typeof res?.count === 'number') {
      total = res.count;
    }
    if (children.length < pageSize || (total !== undefined && page * pageSize >= total)) {
      return { items, total, complete: true };
    }
  }

  return { items, total, complete: false };
};

const toKeySet = (ids: ID[]) => new Set(ids.map((id) => String(id)));

const getSelectable = <T extends WithId>(filtered: T[], disabledIds: ID[]) => {
  const disabled = toKeySet(disabledIds);
  return filtered.filter((it) => !disabled.has(String(it.id)));
};

export type SelectAllState = 'all' | 'some' | 'none';

export const getSelectAllState = <T extends WithId>(selected: WithId[], filtered: T[], disabledIds: ID[] = []) => {
  const selectable = getSelectable(filtered, disabledIds);
  if (!selectable.length) return 'none' as SelectAllState;
  const selectedKeys = toKeySet(selected.map((it) => it.id));
  const count = selectable.filter((it) => selectedKeys.has(String(it.id))).length;
  if (count === 0) return 'none' as SelectAllState;
  return (count === selectable.length ? 'all' : 'some') as SelectAllState;
};

/**
 * Toggles all filtered (non-disabled) items: if every one of them is already selected they get
 * removed from the selection, otherwise the missing ones are appended. Selected items outside of
 * the filtered list are always kept.
 */
export const toggleAllFiltered = <S extends WithId, T extends WithId>(
  selected: S[],
  filtered: T[],
  disabledIds: ID[] = [],
): Array<S | T> => {
  const selectable = getSelectable(filtered, disabledIds);
  const selectableKeys = toKeySet(selectable.map((it) => it.id));

  if (getSelectAllState(selected, filtered, disabledIds) === 'all') {
    return selected.filter((it) => !selectableKeys.has(String(it.id)));
  }

  const selectedKeys = toKeySet(selected.map((it) => it.id));
  return [...selected, ...selectable.filter((it) => !selectedKeys.has(String(it.id)))];
};
