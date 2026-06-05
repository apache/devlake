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

import { useState, useEffect, useMemo } from 'react';
import { CheckCircleFilled, SearchOutlined } from '@ant-design/icons';
import { Space, Tag, Button, Input, Modal, message } from 'antd';
import type { McsID, McsItem, McsColumn } from 'miller-columns-select';
import { MillerColumnsSelect } from 'miller-columns-select';
import { useDebounce } from 'ahooks';

import API from '@/api';
import { Loading, Block, Message } from '@/components';
import { IPluginConfig } from '@/types';
import { getPluginScopeName } from '@/plugins';

import * as T from './types';
import * as S from './styled';

interface Props {
  mode: 'single' | 'multiple';
  plugin: string;
  connectionId: ID;
  config: IPluginConfig['dataScope'];
  disabledScope: any[];
  selectedScope: any[];
  onChange: (selectedScope: any[]) => void;
}

let canceling = false;

export const SearchLocal = ({ mode, plugin, connectionId, config, disabledScope, selectedScope, onChange }: Props) => {
  const [miller, setMiller] = useState<{
    items: McsItem<T.ResItem>[];
    loadedIds: ID[];
    expandedIds: ID[];
    errorId?: ID | null;
    nextTokenMap: Record<ID, string>;
  }>({
    items: [],
    loadedIds: [],
    expandedIds: [],
    nextTokenMap: {},
  });

  const [open, setOpen] = useState(false);
  const [status, setStatus] = useState('init');

  const [query, setQuery] = useState('');
  const search = useDebounce(query, { wait: 500 });

  const scopes = useMemo(
    () =>
      search
        ? miller.items
            .filter((it) => it.name.toLocaleLowerCase().includes(search.toLocaleLowerCase()))
            .filter((it) => it.type !== 'group')
            .map((it) => ({
              ...it,
              parentId: null,
            }))
        : miller.items,
    [search, miller.items],
  );

  const getItems = async ({
    groupId,
    currentPageToken,
    loadAll,
  }: {
    groupId: ID | null;
    currentPageToken?: string;
    loadAll?: boolean;
  }) => {
    if (canceling) {
      canceling = false;
      setStatus('init');
      return;
    }

    let newItems: McsItem<T.ResItem>[] = [];
    let nextPageToken = '';
    let errorId: ID | null;

    try {
      const res = await API.scope.remote(plugin, connectionId, {
        groupId,
        pageToken: currentPageToken,
      });

      newItems = (res.children ?? []).map((it) => ({
        ...it,
        title: it.name,
      }));

      nextPageToken = res.nextPageToken;
    } catch (err: any) {
      errorId = groupId;
      message.error(err.response.data.message);
    }

    if (nextPageToken) {
      setMiller((m) => ({
        ...m,
        items: [...m.items, ...newItems],
        expandedIds: [...m.expandedIds, groupId ?? 'root'],
        nextTokenMap: {
          ...m.nextTokenMap,
          [`${groupId ? groupId : 'root'}`]: nextPageToken,
        },
      }));

      if (loadAll) {
        await getItems({ groupId, currentPageToken: nextPageToken, loadAll });
      }
    } else {
      setMiller((m) => ({
        ...m,
        items: [...m.items, ...newItems],
        expandedIds: [...m.expandedIds, groupId ?? 'root'],
        loadedIds: [...m.loadedIds, groupId ?? 'root'],
        errorId,
      }));

      const groupItems = newItems.filter((it) => it.type === 'group');

      if (loadAll && groupItems.length) {
        groupItems.forEach(async (it) => await getItems({ groupId: it.id, loadAll: true }));
      }
    }
  };

  useEffect(() => {
    getItems({ groupId: null });
  }, []);

  useEffect(() => {
    if (
      miller.items.length &&
      !miller.items.filter((it) => it.type === 'group' && !miller.loadedIds.includes(it.id)).length
    ) {
      setStatus('loaded');
    }
  }, [miller]);

  const handleLoadAllScopes = async () => {
    setOpen(false);
    setStatus('loading');

    if (!miller.loadedIds.includes('root')) {
      await getItems({
        groupId: null,
        currentPageToken: miller.nextTokenMap['root'],
        loadAll: true,
      });
    }

    const noLoadedItems = miller.items.filter((it) => it.type === 'group' && !miller.loadedIds.includes(it.id));
    if (noLoadedItems.length) {
      noLoadedItems.forEach(async (it) => {
        await getItems({
          groupId: it.id,
          currentPageToken: miller.nextTokenMap[it.id],
          loadAll: true,
        });
      });
    }
  };

  const handleCancelLoadAllScopes = () => {
    setStatus('cancel');
    canceling = true;
  };

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
        {(status === 'loading' || status === 'cancel') && (
          <S.JobLoad>
            <Loading style={{ marginRight: 8 }} size={20} />
            Loading: <span className="count">{miller.items.length}</span> scopes found
            <Button style={{ marginLeft: 8 }} loading={status === 'cancel'} onClick={handleCancelLoadAllScopes}>
              Cancel
            </Button>
          </S.JobLoad>
        )}

        {status === 'loaded' && (
          <S.JobLoad>
            <CheckCircleFilled style={{ color: 'var(--devlake-color-success)' }} />
            <span className="count">{miller.items.length}</span> scopes found
          </S.JobLoad>
        )}

        {status === 'init' && (
          <S.JobLoad>
            <Button type="primary" disabled={!miller.items.length} onClick={() => setOpen(true)}>
              Load all scopes to search by keywords
            </Button>
          </S.JobLoad>
        )}
      </Block>
      <Block>
        {status === 'loaded' && (
          <Input prefix={<SearchOutlined />} value={query} onChange={(e) => setQuery(e.target.value)} />
        )}
        <MillerColumnsSelect
          mode={mode}
          items={scopes}
          columnCount={search ? 1 : config.millerColumn?.columnCount ?? 1}
          columnHeight={300}
          getCanExpand={(it) => it.type === 'group'}
          getHasMore={(id) => !miller.loadedIds.includes(id ?? 'root')}
          getHasError={(id) => id === miller.errorId}
          onExpand={(id: McsID) => getItems({ groupId: id })}
          onScroll={(id: McsID | null) =>
            getItems({ groupId: id, currentPageToken: miller.nextTokenMap[id ?? 'root'] })
          }
          renderTitle={(column: McsColumn) =>
            !column.parentId &&
            config.millerColumn?.firstColumnTitle && (
              <S.ColumnTitle>{config.millerColumn.firstColumnTitle}</S.ColumnTitle>
            )
          }
          renderLoading={() => <Loading size={20} style={{ padding: '4px 12px' }} />}
          renderError={() => <span style={{ color: 'red' }}>Something Error</span>}
          disabledIds={(disabledScope ?? []).map((it) => it.id)}
          selectedIds={selectedScope.map((it) => it.id)}
          onSelectItemIds={(selectedIds: ID[]) => onChange(miller.items.filter((it) => selectedIds.includes(it.id)))}
          expandedIds={miller.expandedIds}
        />
      </Block>
      <Modal open={open} centered onOk={handleLoadAllScopes} onCancel={() => setOpen(false)}>
        <Message content={`This operation may take a long time, as it iterates through all the ${config.title}.`} />
      </Modal>
    </>
  );
};
