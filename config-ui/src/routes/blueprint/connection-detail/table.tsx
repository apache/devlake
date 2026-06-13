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
import { AppstoreAddOutlined } from '@ant-design/icons';
import { Table, Button, Flex, Modal, Tag, message } from 'antd';

import API from '@/api';
import { useRefreshData } from '@/hooks';
import { getPluginConfig, getPluginScopeId, getPluginScopeName, ScopeConfig, ScopeConfigSelect } from '@/plugins';
import { operator } from '@/utils';

interface Props {
  plugin: string;
  connectionId: ID;
  scopeIds: ID[];
}

export const BlueprintConnectionDetailTable = ({ plugin, connectionId, scopeIds }: Props) => {
  const [version, setVersion] = useState(1);
  const [selectedScopeIds, setSelectedScopeIds] = useState<ID[]>([]);
  const [bulkModalOpen, setBulkModalOpen] = useState(false);
  const [bulkOperating, setBulkOperating] = useState(false);

  const pluginConfig = getPluginConfig(plugin);

  const { ready, data } = useRefreshData(async () => {
    const scopes = await Promise.all(scopeIds.map((scopeId) => API.scope.get(plugin, connectionId, scopeId)));
    return scopes.map((sc) => ({
      id: getPluginScopeId(plugin, sc.scope),
      name: getPluginScopeName(plugin, sc.scope) || sc.scope.fullName || sc.scope.name,
      scopeConfigId: sc.scopeConfig?.id,
      scopeConfigName: sc.scopeConfig?.name,
    }));
  }, [version]);

  const handleRefresh = () => {
    setSelectedScopeIds([]);
    setVersion((v) => v + 1);
  };
  
  // Apply one scope config to all selected data scopes in bulk
  const handleBulkApplyScopeConfig = async (scopeConfigId: ID) => {
    setBulkOperating(true);
    const configId = scopeConfigId === 'None' ? null : +scopeConfigId;
    const [success] = await operator(
      () =>
        Promise.all(
          selectedScopeIds.map((scopeId) =>
            API.scope.update(plugin, connectionId, scopeId, { scopeConfigId: configId }),
          ),
        ),
      { setOperating: setBulkOperating },
    );
    if (success) {
      message.success(`Scope config applied to ${selectedScopeIds.length} data scope(s).`);
      setBulkModalOpen(false);
      handleRefresh();
    }
  };

  const hasScopeConfig = !!pluginConfig.scopeConfig;

  return (
    <>
      {/* Bulk toolbar — shown only when rows are selected and plugin supports scope configs */}
      {hasScopeConfig && selectedScopeIds.length > 0 && (
        <Flex align="center" gap="small" style={{ marginBottom: 12 }}>
          <Tag color="blue">{selectedScopeIds.length} selected</Tag>
          <Button
            type="primary"
            icon={<AppstoreAddOutlined />}
            loading={bulkOperating}
            onClick={() => setBulkModalOpen(true)}
          >
            Apply Scope Config to Selected
          </Button>
          <Button onClick={() => setSelectedScopeIds([])}>Clear Selection</Button>
        </Flex>
      )}

      <Table
        loading={!ready}
        rowKey="id"
        size="middle"
        rowSelection={
          hasScopeConfig
            ? {
                type: 'checkbox',
                selectedRowKeys: selectedScopeIds,
                onChange: (keys) => setSelectedScopeIds(keys as ID[]),
              }
            : undefined
        }
        columns={[
          {
            title: 'Data Scope',
            dataIndex: 'name',
            key: 'name',
          },
          {
            title: 'Scope Config',
            key: 'scopeConfig',
            render: (_, { id, name, scopeConfigId, scopeConfigName }) => (
              <ScopeConfig
                plugin={plugin}
                connectionId={connectionId}
                scopeId={id}
                scopeName={name}
                scopeConfigId={scopeConfigId}
                scopeConfigName={scopeConfigName}
                onSuccess={handleRefresh}
              />
            ),
          },
        ]}
        dataSource={data ?? []}
      />
      {/* Bulk scope config picker modal */}
      <Modal
        destroyOnClose
        open={bulkModalOpen}
        width={960}
        centered
        footer={null}
        title={`Apply Scope Config to ${selectedScopeIds.length} Selected Data Scope(s)`}
        onCancel={() => setBulkModalOpen(false)}
      >
        <ScopeConfigSelect
          plugin={plugin}
          connectionId={connectionId}
          onCancel={() => setBulkModalOpen(false)}
          onSubmit={handleBulkApplyScopeConfig}
        />
      </Modal>
    </>
  );
};
