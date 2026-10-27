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

import { useEffect, useMemo, useState } from 'react';
import { SearchOutlined } from '@ant-design/icons';
import { Input, Space, Tag, message } from 'antd';

import API from '@/api';
import { Block, Loading } from '@/components';

const ROOT_GROUP_ID = 'root';

interface ProjectScopeData {
  id: string;
  name?: string;
  identifier?: string;
}

interface ScopeItem {
  id: string;
  name: string;
  data: ProjectScopeData;
}

interface Props {
  connectionId: ID;
  disabledItems?: Array<{ id: ID }>;
  selectedItems: ScopeItem[];
  onChangeSelectedItems: (items: ScopeItem[]) => void;
}

export const RedmineDataScope = ({
  connectionId,
  disabledItems = [],
  selectedItems,
  onChangeSelectedItems,
}: Props) => {
  const [loading, setLoading] = useState(true);
  const [items, setItems] = useState<ScopeItem[]>([]);
  const [query, setQuery] = useState('');

  const disabledIds = useMemo(() => new Set(disabledItems.map((it) => `${it.id}`)), [disabledItems]);

  useEffect(() => {
    let cancelled = false;

    const loadProjects = async () => {
      setLoading(true);
      try {
        const res = await API.scope.remote('redmine', connectionId, { groupId: ROOT_GROUP_ID });
        if (cancelled) {
          return;
        }
        const scopes = (res.children ?? [])
          .filter((it) => it.type === 'scope')
          .map((it) => ({
            id: `${it.id}`,
            name: it.name,
            data: it.data as ProjectScopeData,
          }));
        setItems(scopes);
      } catch (err: any) {
        if (!cancelled) {
          message.error(err?.response?.data?.message ?? 'Failed to load Redmine projects');
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    loadProjects();

    return () => {
      cancelled = true;
    };
  }, [connectionId]);

  const filteredItems = useMemo(() => {
    const term = query.trim().toLowerCase();
    if (!term) {
      return items;
    }
    return items.filter((item) => {
      const identifier = item.data?.identifier ?? '';
      return item.name.toLowerCase().includes(term) || identifier.toLowerCase().includes(term);
    });
  }, [items, query]);

  return (
    <>
      <Block title="Projects" required>
        <Space wrap>
          {selectedItems.length ? (
            selectedItems.map((item) => (
              <Tag
                key={item.id}
                color="blue"
                closable
                onClose={() => onChangeSelectedItems(selectedItems.filter((it) => it.id !== item.id))}
              >
                {item.name || item.id}
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
          placeholder="Search projects…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        {loading ? (
          <Loading style={{ marginTop: 12 }} />
        ) : (
          <div style={{ marginTop: 12, maxHeight: 320, overflow: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr>
                  <th style={{ width: 40, padding: '8px 4px' }} />
                  <th style={{ textAlign: 'left', padding: '8px 4px' }}>Project</th>
                </tr>
              </thead>
              <tbody>
                {filteredItems.map((item) => {
                  const checked = selectedItems.some((it) => it.id === item.id);
                  const disabled = disabledIds.has(item.id);
                  return (
                    <tr key={item.id} style={{ borderTop: '1px solid var(--devlake-color-border)' }}>
                      <td style={{ padding: '8px 4px' }}>
                        <input
                          type="checkbox"
                          checked={checked}
                          disabled={disabled}
                          onChange={(e) => {
                            if (e.target.checked) {
                              onChangeSelectedItems([...selectedItems, item]);
                            } else {
                              onChangeSelectedItems(selectedItems.filter((it) => it.id !== item.id));
                            }
                          }}
                        />
                      </td>
                      <td style={{ padding: '8px 4px' }}>{item.name}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            {!filteredItems.length && <div style={{ padding: '12px 4px' }}>No projects found</div>}
          </div>
        )}
      </Block>
    </>
  );
};
