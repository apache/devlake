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
import { PlusOutlined } from '@ant-design/icons';
import { Flex, Table, Button, Modal } from 'antd';

import API from '@/api';
import { useRefreshData } from '@/hooks';

import { ScopeConfigForm } from '../scope-config-form';

interface Props {
  plugin: string;
  connectionId: ID;
  scopeConfigId?: ID;
  onCancel: () => void;
  onSubmit: (trId: ID) => void;
}

export const ScopeConfigSelect = ({ plugin, connectionId, scopeConfigId, onCancel, onSubmit }: Props) => {
  const [version, setVersion] = useState(1);
  const [trId, setTrId] = useState<ID>();
  const [open, setOpen] = useState(false);
  const [showAll, setShowAll] = useState(false);
  
  const { ready, data } = useRefreshData(
    () => showAll ? API.scopeConfig.listAll(plugin) : API.scopeConfig.list(plugin, connectionId),
    [version, showAll],
  );

  const dataSource = useMemo(
    () => (data ? (scopeConfigId ? [{ id: 'None', name: 'No Scope Config' }].concat(data) : data) : []),
    [data, scopeConfigId],
  );
  
  const defaultName = useMemo(() => `shared-config-<${(data ?? []).length}>`, [data]);

  useEffect(() => {
    setTrId(scopeConfigId);
  }, [scopeConfigId]);

  const handleShowDialog = () => {
    setOpen(true);
  };

  const handleHideDialog = () => {
    setOpen(false);
  };

  const handleSubmit = async (trId: ID) => {
    handleHideDialog();
    setVersion((v) => v + 1);
    setTrId(trId);
  };

  return (
    <Flex vertical gap="middle">
      <Flex style={{ marginTop: 20 }} gap="small" align="center">
        <Button type="primary" icon={<PlusOutlined />} onClick={handleShowDialog}>
          Add New Scope Config
        </Button>
        <Button
          type={showAll ? 'primary' : 'default'}
          onClick={() => setShowAll((v) => !v)}
        >
          {showAll ? 'Showing: All Connections' : 'This Connection Only'}
        </Button>
      </Flex>
      <Table
        rowKey="id"
        size="small"
        loading={!ready}
        columns={[{ title: 'Name', dataIndex: 'name', key: 'name' }]}
        dataSource={dataSource}
        rowSelection={{
          type: 'radio',
          selectedRowKeys: trId ? [trId] : [],
          onChange: (selectedRowKeys) => setTrId(selectedRowKeys[0] as ID),
        }}
        pagination={false}
      />
      <Flex justify="flex-end" gap="small">
        <Button style={{}} onClick={onCancel}>
          Cancel
        </Button>
        <Button type="primary" disabled={!trId} onClick={() => trId && onSubmit?.(trId)}>
          Save
        </Button>
      </Flex>
      <Modal
        destroyOnClose
        open={open}
        width={960}
        centered
        footer={null}
        title="Add Scope Config"
        onCancel={handleHideDialog}
      >
        <ScopeConfigForm
          plugin={plugin}
          connectionId={connectionId}
          defaultName={defaultName}
          onCancel={handleHideDialog}
          onSubmit={handleSubmit}
        />
      </Modal>
    </Flex>
  );
};
