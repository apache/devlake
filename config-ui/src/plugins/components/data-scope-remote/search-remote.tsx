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

import { useState, useEffect, useMemo, useRef } from 'react';
import { SearchOutlined } from '@ant-design/icons';
import { Space, Tag, Input, Checkbox, Spin, message } from 'antd';
import type { McsID, McsItem, McsColumn } from 'miller-columns-select';
import MillerColumnsSelect from 'miller-columns-select';
import { useDebounce } from 'ahooks';
import { uniqBy } from 'lodash';

import API from '@/api';
import { Loading, Block } from '@/components';
import { IPluginConfig } from '@/types';
import { getPluginScopeName } from '@/plugins';

import * as T from './types';
import * as S from './styled';
import { fetchAllSearchResults, getSelectAllState, toggleAllFiltered } from './select-all';

interface Props {
  mode: 'single' | 'multiple';
  plugin: string;
  connectionId: ID;
  config: IPluginConfig['dataScope'];
  disabledScope: any[];
  selectedScope: any[];
  onChange: (selectedScope: any[]) => void;
}

export const SearchRemote = ({ mode, plugin, connectionId, config, disabledScope, selectedScope, onChange }: Props) => {
  const [miller, setMiller] = useState<{
    items: McsItem<T.ResItem>[];
    loadedIds: ID[];
    errorId?: ID | null;
    nextTokenMap: Record<ID, string>;
  }>({
    items: [],
    loadedIds: [],
    nextTokenMap: {},
  });

  const PAGE_SIZE = 50;

  const [search, setSearch] = useState<{
    loading: boolean;
    items: McsItem<T.ResItem>[];
    currentItems: McsItem<T.ResItem>[];
    query: string;
    page: number;
    total: number;
    hasMore: boolean;
  }>({
    loading: true,
    items: [],
    currentItems: [],
    query: '',
    page: 1,
    total: 0,
    hasMore: false,
  });

  const searchDebounce = useDebounce(search.query, { wait: 500 });

  const allItems = useMemo(
    () =>
      uniqBy(
        [...miller.items, ...search.items].filter((it) => it.type === 'scope'),
        'id',
      ),
    [miller.items, search.items],
  );

  const getItems = async (groupId: ID | null, currentPageToken?: string) => {
    let newItems: McsItem<T.ResItem>[] = [];
    let nextPageToken = '';
    let errorId: ID | null;

    try {
      const res = await API.scope.remote(plugin, connectionId, {
        groupId,
        pageToken: currentPageToken,
      });

      newItems = (res?.children ?? []).map((it) => ({
        ...it,
        title: getPluginScopeName(plugin, it) || it.name,
      }));

      nextPageToken = res.nextPageToken;
    } catch (err: any) {
      errorId = groupId;
      message.error(err.response.data.message);
    }

    if (nextPageToken && newItems.length) {
      setMiller((m) => ({
        ...m,
        items: [...m.items, ...newItems],
        nextTokenMap: {
          ...m.nextTokenMap,
          [`${groupId ? groupId : 'root'}`]: nextPageToken,
        },
      }));
    } else {
      setMiller((m) => ({
        ...m,
        items: [...m.items, ...newItems],
        loadedIds: [...m.loadedIds, groupId ?? 'root'],
        errorId,
      }));
    }
  };

  useEffect(() => {
    getItems(null);
  }, []);

  const searchItems = async () => {
    if (!searchDebounce) return;

    try {
      const res = await API.scope.searchRemote(plugin, connectionId, {
        search: searchDebounce,
        page: search.page,
        pageSize: PAGE_SIZE,
      });

      const newItems = (res?.children ?? []).map((it) => ({
        ...it,
        title: getPluginScopeName(plugin, it) || it.fullName || it.name,
      }));

      const total = res.count ?? 0;
      // If the backend returns a real total, use it; otherwise fall back to
      // the heuristic: a full page means there are likely more results.
      const hasMore = total > 0 ? search.page * PAGE_SIZE < total : newItems.length >= PAGE_SIZE;

      setSearch((s) => ({
        ...s,
        loading: false,
        items: [...allItems, ...newItems],
        // Accumulate results across pages so previous pages remain visible
        currentItems: s.page === 1 ? newItems : uniqBy([...s.currentItems, ...newItems], 'id'),
        total,
        hasMore,
      }));
    } catch {
      setSearch((s) => ({ ...s, loading: false, hasMore: false }));
    }
  };

  useEffect(() => {
    searchItems();
  }, [searchDebounce, search.page]);

  // ---- "ALL" toggle for the filtered search result (opt-in via config.searchSelectAll) ----
  // It is rendered as the first entry of the result list (like `showSelectAll` of miller-columns-select).
  // The built-in `showSelectAll` can't be used here: it only covers the already loaded page(s) and
  // replaces the whole selection, while this toggle loads every page of the search result and keeps
  // the selection outside of the filter.
  const showSelectAll = !!config.searchSelectAll && mode === 'multiple' && !!searchDebounce;
  const disabledIds = useMemo(() => (disabledScope ?? []).map((it) => it.id), [disabledScope]);
  const [fullResult, setFullResult] = useState<{ query: string; items: McsItem<T.ResItem>[] } | null>(null);
  const [selectAllLoading, setSelectAllLoading] = useState(false);
  const latestQueryRef = useRef(searchDebounce);
  latestQueryRef.current = searchDebounce;

  // A new search invalidates a previously loaded complete result.
  useEffect(() => {
    setFullResult(null);
  }, [searchDebounce]);

  // The complete filtered list is known if it was fully fetched or no further pages exist.
  const knownFilteredItems = useMemo(() => {
    if (fullResult && fullResult.query === searchDebounce) return fullResult.items;
    if (!search.loading && !search.hasMore) return search.currentItems;
    return null;
  }, [fullResult, searchDebounce, search.loading, search.hasMore, search.currentItems]);

  // Number of matches: reported by the backend (`count`) or – once fully loaded – the list length.
  const matchCount = search.total > 0 ? search.total : knownFilteredItems?.length;

  const selectAllState = useMemo(() => {
    const state = getSelectAllState(selectedScope, knownFilteredItems ?? search.currentItems, disabledIds);
    // Without the complete list we can never claim that "all" are selected.
    return state === 'all' && !knownFilteredItems ? 'some' : state;
  }, [selectedScope, knownFilteredItems, search.currentItems, disabledIds]);

  const selectAllDisabled = selectAllLoading || search.loading || knownFilteredItems?.length === 0;

  const handleToggleAll = async () => {
    if (selectAllDisabled) return;

    let filtered = knownFilteredItems;

    if (!filtered) {
      const query = searchDebounce;
      let complete = true;
      setSelectAllLoading(true);
      try {
        const res = await fetchAllSearchResults((page, pageSize) =>
          API.scope.searchRemote(plugin, connectionId, { search: query, page, pageSize }),
        );
        complete = res.complete;
        filtered = res.items.map((it) => ({
          ...it,
          title: getPluginScopeName(plugin, it) || it.fullName || it.name,
        })) as McsItem<T.ResItem>[];
      } catch (err: any) {
        message.error(err?.response?.data?.message ?? 'Failed to load all search results.');
        return;
      } finally {
        setSelectAllLoading(false);
      }

      // The user changed the search while we were loading – discard the stale result.
      if (latestQueryRef.current !== query) return;

      const loaded = filtered;
      if (complete) {
        setFullResult({ query, items: loaded });
      } else {
        message.warning(
          `Only the first ${loaded.length} matches could be loaded. Please refine the search to select the rest.`,
        );
      }
      setSearch((s) => ({
        ...s,
        items: uniqBy([...s.items, ...loaded], 'id'),
        currentItems: uniqBy([...s.currentItems, ...loaded], 'id'),
        hasMore: !complete,
      }));
    }

    onChange(toggleAllFiltered(selectedScope, filtered, disabledIds));
  };

  const renderSelectAll = () =>
    showSelectAll &&
    search.currentItems.length > 0 && (
      <S.SelectAllItem
        data-testid="search-select-all"
        $disabled={selectAllDisabled}
        onClick={(e) => {
          // the checkbox handles its own clicks
          if ((e.target as HTMLElement).closest('.ant-checkbox-wrapper')) return;
          handleToggleAll();
        }}
      >
        <Checkbox
          checked={selectAllState === 'all'}
          indeterminate={selectAllState === 'some'}
          disabled={selectAllDisabled}
          onChange={handleToggleAll}
        >
          {matchCount !== undefined ? `ALL (${matchCount})` : 'ALL'}
        </Checkbox>
        {selectAllLoading && <Spin size="small" />}
      </S.SelectAllItem>
    );

  return (
    <>
      <Block title={config.title} required>
        <Space wrap>
          {selectedScope.length ? (
            selectedScope.map((sc) => (
              <Tag
                key={sc.id}
                color="blue"
                closable
                onClose={() => onChange(selectedScope.filter((it) => it.id !== sc.id))}
              >
                {getPluginScopeName(plugin, sc) || sc.fullName || sc.name || sc.id}
              </Tag>
            ))
          ) : (
            <span>Please select scope...</span>
          )}
        </Space>
      </Block>
      <Block>
        <Input
          prefix={<SearchOutlined />}
          placeholder={config.searchPlaceholder ?? 'Search'}
          value={search.query}
          onChange={(e) =>
            setSearch({ ...search, query: e.target.value, loading: true, currentItems: [], page: 1, hasMore: false })
          }
        />
        {!searchDebounce ? (
          <MillerColumnsSelect
            mode={mode}
            items={miller.items}
            columnCount={config.millerColumn?.columnCount ?? 1}
            columnHeight={300}
            getCanExpand={(it) => it.type === 'group'}
            getHasMore={(id) => !miller.loadedIds.includes(id ?? 'root')}
            getHasError={(id) => id === miller.errorId}
            onExpand={(id: McsID) => getItems(id, miller.nextTokenMap[id])}
            onScroll={(id: McsID | null) => getItems(id, miller.nextTokenMap[id ?? 'root'])}
            renderTitle={(column: McsColumn) =>
              !column.parentId &&
              config.millerColumn?.firstColumnTitle && (
                <S.ColumnTitle>{config.millerColumn.firstColumnTitle}</S.ColumnTitle>
              )
            }
            renderLoading={() => <Loading size={20} style={{ padding: '4px 12px' }} />}
            renderError={() => <span style={{ color: 'red' }}>Something Error</span>}
            disabledIds={disabledIds}
            selectedIds={selectedScope.map((it) => it.id)}
            onSelectItemIds={(selectedIds: ID[]) => onChange(allItems.filter((it) => selectedIds.includes(it.id)))}
          />
        ) : (
          <MillerColumnsSelect
            mode={mode}
            items={search.currentItems}
            columnCount={1}
            columnHeight={300}
            getCanExpand={() => false}
            getHasMore={() => search.hasMore}
            onScroll={() => {
              if (!search.loading && search.hasMore) {
                setSearch((s) => ({ ...s, loading: true, page: s.page + 1 }));
              }
            }}
            renderTitle={renderSelectAll}
            renderLoading={() => <Loading size={20} style={{ padding: '4px 12px' }} />}
            disabledIds={disabledIds}
            selectedIds={selectedScope.map((it) => it.id)}
            onSelectItemIds={(selectedIds: ID[]) => onChange(allItems.filter((it) => selectedIds.includes(it.id)))}
          />
        )}
      </Block>
    </>
  );
};
