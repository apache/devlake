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

import { useState, useMemo } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { PlusOutlined, SettingOutlined } from '@ant-design/icons';
import { Flex, Table, Modal, Radio, Button, Input, Tag } from 'antd';
import dayjs from 'dayjs';

import API from '@/api';
import { PageHeader, Block, TextTooltip, IconButton } from '@/components';
import { getCronOptions, cronPresets, getCron, PATHS } from '@/config';
import { ConnectionName } from '@/features';
import { useRefreshData } from '@/hooks';
import { IBlueprint, IBPMode } from '@/types';
import { formatTime, operator } from '@/utils';

import * as S from './styled';

export const BlueprintHomePage = () => {
  const [version, setVersion] = useState(1);
  const [type, setType] = useState('all');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [mode, setMode] = useState(IBPMode.NORMAL);
  const [saving, setSaving] = useState(false);

  const navigate = useNavigate();

  const { ready, data } = useRefreshData(
    () => API.blueprint.list({ type: type.toLocaleUpperCase(), page, pageSize }),
    [version, type, page, pageSize],
  );

  const [options, presets] = useMemo(() => [getCronOptions(), cronPresets.map((preset) => preset.config)], []);
  const [dataSource, total] = useMemo(() => [data?.blueprints ?? [], data?.count ?? 0], [data]);

  const handleShowDialog = () => setOpen(true);
  const handleHideDialog = () => {
    setName('');
    setMode(IBPMode.NORMAL);
    setOpen(false);
  };

  const handleCreate = async () => {
    const payload: any = {
      name,
      mode,
      enable: true,
      cronConfig: presets[0],
      isManual: false,
      skipOnFail: true,
    };

    if (mode === IBPMode.NORMAL) {
      payload.timeAfter = formatTime(dayjs().subtract(6, 'month').startOf('day').toDate(), 'YYYY-MM-DD[T]HH:mm:ssZ');
      payload.connections = [];
    }

    if (mode === IBPMode.ADVANCED) {
      payload.timeAfter = undefined;
      payload.connections = undefined;
      payload.plan = [[]];
    }

    const [success] = await operator(() => API.blueprint.create(payload), {
      setOperating: setSaving,
    });

    if (success) {
      handleHideDialog();
      setVersion((v) => v + 1);
    }
  };

  return (
    <PageHeader
      breadcrumbs={[
        { name: 'Advanced', path: PATHS.BLUEPRINTS() },
        { name: 'Blueprints', path: PATHS.BLUEPRINTS() },
      ]}
      description="This is a complete list of all Blueprints you have created, whether they belong to Projects or not."
    >
      <Flex vertical gap="middle">
        <Flex justify="space-between">
          <Radio.Group optionType="button" value={type} onChange={({ target: { value } }) => setType(value)}>
            <Radio value="all">All</Radio>
            {options.map(({ label }) => (
              <Radio key={label} value={label}>
                {label}
              </Radio>
            ))}
          </Radio.Group>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleShowDialog}>
            New Blueprint
          </Button>
        </Flex>
        <Table
          rowKey="id"
          size="middle"
          loading={!ready}
          columns={[
            {
              title: 'Blueprint Name',
              key: 'name',
              render: (_, { id, name }) => (
                <Link
                  to={PATHS.BLUEPRINT(id)}
                  state={{ activeKey: 'configuration' }}
                  style={{ color: 'var(--devlake-color-text)' }}
                >
                  <TextTooltip content={name}>{name}</TextTooltip>
                </Link>
              ),
            },
            {
              title: 'Data Connections',
              key: 'connections',
              render: (_, { mode, connections }: Pick<IBlueprint, 'mode' | 'connections'>) => {
                if (mode === IBPMode.ADVANCED) {
                  return 'Advanced Mode';
                }

                if (!connections.length) {
                  return 'N/A';
                }

                return (
                  <ul>
                    {connections.map((it) => (
                      <li key={`${it.pluginName}-${it.connectionId}`}>
                        <ConnectionName plugin={it.pluginName} connectionId={it.connectionId} />
                      </li>
                    ))}
                  </ul>
                );
              },
            },
            {
              title: 'Frequency',
              key: 'frequency',
              render: (_, { isManual, cronConfig }) => {
                const cron = getCron(isManual, cronConfig);
                return cron.label;
              },
            },
            {
              title: 'Next Run Time',
              key: 'nextRunTime',
              render: (_, { isManual, cronConfig }) => {
                const cron = getCron(isManual, cronConfig);
                return formatTime(cron.nextTime);
              },
            },
            {
              title: 'Project',
              dataIndex: 'projectName',
              key: 'project',
              render: (val) =>
                val ? (
                  <Link to={PATHS.PROJECT(val)}>
                    <TextTooltip content={val}>{val}</TextTooltip>
                  </Link>
                ) : (
                  'N/A'
                ),
            },
            {
              title: 'Status',
              dataIndex: 'enable',
              key: 'enable',
              align: 'center',
              render: (val) => <Tag color={val ? 'blue' : 'red'}>{val ? 'Enabled' : 'Disabled'}</Tag>,
            },
            {
              title: '',
              dataIndex: 'id',
              key: 'action',
              width: 100,
              align: 'center',
              render: (val) => (
                <IconButton
                  type="primary"
                  icon={<SettingOutlined />}
                  helptip="Blueprint Configuration"
                  onClick={() =>
                    navigate(PATHS.BLUEPRINT(val), {
                      state: {
                        activeKey: 'configuration',
                      },
                    })
                  }
                />
              ),
            },
          ]}
          dataSource={dataSource}
          pagination={{
            current: page,
            pageSize,
            total,
            onChange: ((newPage: number, newPageSize: number) => {
              setPage(newPage);
              if (newPageSize !== pageSize) {
                setPageSize(newPageSize);
              }
            }) as (newPage: number) => void,
          }}
        />
      </Flex>
      <Modal
        open={open}
        width={820}
        centered
        title="Create a New Blueprint"
        okText="Save"
        okButtonProps={{
          disabled: !name,
          loading: saving,
        }}
        onOk={handleCreate}
        onCancel={handleHideDialog}
      >
        <S.DialogWrapper>
          <Block
            title="Blueprint Name"
            description="Give your Blueprint a unique name to help you identify it in the future."
            required
          >
            <Input
              style={{ width: 386 }}
              placeholder="Your Blueprint Name"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </Block>
          <Block
            title="Blueprint Mode"
            description="Normal Mode is usually adequate for most usages. But if you need to customize how tasks are executed in
            the Blueprint, please use Advanced Mode to create a Blueprint."
            required
          >
            <Radio.Group value={mode} onChange={({ target: { value } }) => setMode(value)}>
              <Radio value={IBPMode.NORMAL}>Normal Mode</Radio>
              <Radio value={IBPMode.ADVANCED}>Advanced Mode</Radio>
            </Radio.Group>
          </Block>
        </S.DialogWrapper>
      </Modal>
    </PageHeader>
  );
};
