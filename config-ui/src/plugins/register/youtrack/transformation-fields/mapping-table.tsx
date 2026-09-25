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

import { useState } from 'react';
import { Button, Input, Select, Table, Tag, Tooltip } from 'antd';

// UnionRow is one line of the union-by-name mapping table. `presentIn` holds
// the labels of the attached projects whose bundle contains the value;
// synonyms (Subtask vs Sub-task) stay separate rows because mappings are
// name-keyed.
export type UnionRow = {
  name: string;
  presentIn: string[];
};

const SCROLL_THRESHOLD = 30;
const SCROLL_HEIGHT = 420;

interface Props {
  title: string;
  description?: React.ReactNode;
  projectLabels: string[];
  rows: UnionRow[];
  staleNames: string[];
  mappings: Record<string, string>;
  options: Array<{ label: string; value: string }>;
  unmappedOptionLabel?: (name: string) => string;
  autoSeeds?: Record<string, string>;
  manualMode: boolean;
  emptyHint?: string;
  onChange: (name: string, value?: string) => void;
}

export const MappingTable = ({
  title,
  description,
  projectLabels,
  rows,
  staleNames,
  mappings,
  options,
  unmappedOptionLabel,
  autoSeeds,
  manualMode,
  emptyHint,
  onChange,
}: Props) => {
  const [newName, setNewName] = useState('');
  const [newValue, setNewValue] = useState<string>();

  // stale rows: mapped values gone from every attached bundle — kept in the
  // config (an upstream rename degrades gracefully), but highlighted
  const dataSource: UnionRow[] = [...rows, ...staleNames.map((name) => ({ name, presentIn: [] as string[] }))];

  const selectOptions = (name: string) => [
    ...(unmappedOptionLabel ? [{ label: unmappedOptionLabel(name), value: '' }] : []),
    ...options,
  ];

  const handleAdd = () => {
    const name = newName.trim();
    if (!name || name in mappings) {
      return;
    }
    onChange(name, newValue ?? options[0]?.value);
    setNewName('');
  };

  const columns = [
    {
      title: 'Source value',
      dataIndex: 'name',
      key: 'name',
      width: '34%',
      render: (name: string) => name,
    },
    ...(!manualMode
      ? [
          {
            title: 'Present in',
            key: 'presentIn',
            width: '24%',
            render: (_: unknown, row: UnionRow) =>
              staleNames.includes(row.name) ? (
                <Tooltip title="Mapped value no longer present in any attached project's bundle (renamed upstream?) — kept in the config">
                  <Tag color="error">stale</Tag>
                </Tooltip>
              ) : (
                projectLabels.map((label) => (
                  <Tag key={label} color={row.presentIn.includes(label) ? 'processing' : undefined}>
                    {label}
                  </Tag>
                ))
              ),
          },
        ]
      : []),
    {
      title: 'Standard mapping',
      key: 'mapping',
      render: (_: unknown, row: UnionRow) => (
        <>
          <Select
            style={{ width: 220 }}
            options={selectOptions(row.name)}
            value={mappings[row.name] ?? ''}
            onChange={(value) => onChange(row.name, value || undefined)}
          />
          {staleNames.includes(row.name) && (
            <Tooltip title="Stale — renamed upstream? Kept in the config">
              <Tag color="error" style={{ marginLeft: 4 }}>
                stale
              </Tag>
            </Tooltip>
          )}
          {!staleNames.includes(row.name) && autoSeeds && autoSeeds[row.name] === mappings[row.name] && (
            <Tooltip title="Pre-filled from the bundle's isResolved flag — review before saving; written only when you save">
              <Tag color="success" style={{ marginLeft: 4 }}>
                auto
              </Tag>
            </Tooltip>
          )}
        </>
      ),
    },
  ];

  return (
    <div style={{ marginTop: 24 }}>
      <h3 style={{ marginBottom: 4 }}>{title}</h3>
      {description && <p style={{ color: 'rgba(0, 0, 0, 0.45)', marginBottom: 12 }}>{description}</p>}
      <Table
        rowKey="name"
        size="small"
        columns={columns}
        dataSource={dataSource}
        pagination={false}
        locale={emptyHint ? { emptyText: emptyHint } : undefined}
        scroll={dataSource.length > SCROLL_THRESHOLD ? { y: SCROLL_HEIGHT } : undefined}
      />
      {manualMode && (
        <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
          <Input
            style={{ width: 320 }}
            placeholder="Type the value name as it appears in YouTrack"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onPressEnter={handleAdd}
          />
          <Select
            style={{ width: 220 }}
            options={options}
            value={newValue ?? options[0]?.value}
            onChange={(value) => setNewValue(value)}
          />
          <Button type="primary" ghost disabled={!newName.trim() || newName.trim() in mappings} onClick={handleAdd}>
            Add
          </Button>
        </div>
      )}
    </div>
  );
};
