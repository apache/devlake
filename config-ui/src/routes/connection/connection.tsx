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
import { useParams, useNavigate, Link } from 'react-router-dom';
import { Helmet } from 'react-helmet';
import { DeleteOutlined, PlusOutlined, LinkOutlined, ClearOutlined } from '@ant-design/icons';
import { theme, Space, Table, Button, Modal, message } from 'antd';

import API from '@/api';
import { PageHeader, Message, IconButton } from '@/components';
import { PATHS } from '@/config';
import { useAppDispatch, useAppSelector } from '@/hooks';
import { selectConnection, removeConnection } from '@/features';
import { useRefreshData } from '@/hooks';
import {
  ConnectionStatus,
  DataScopeRemote,
  getPluginConfig,
  getPluginScopeId,
  ScopeConfig,
  ScopeConfigSelect,
} from '@/plugins';
import { IConnection } from '@/types';
import { operator } from '@/utils';

import * as S from './styled';
import { getPluginScopeName } from '@/plugins';

const brandName = import.meta.env.DEVLAKE_BRAND_NAME ?? 'DevLake';

export const Connection = () => {
  const [type, setType] = useState<
    | 'deleteConnection'
    | 'createDataScope'
    | 'clearDataScope'
    | 'deleteDataScope'
    | 'associateScopeConfig'
    | 'deleteConnectionFailed'
    | 'deleteDataScopeFailed'
    | 'deleteSelectedScopes'
    | 'confirmDeleteSelectedScopes'
  >();
  const [operating, setOperating] = useState(false);
  const [version, setVersion] = useState(1);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [scopeId, setScopeId] = useState<ID>();
  const [scopeIds, setScopeIds] = useState<ID[]>([]);
  const [conflict, setConflict] = useState<string[]>([]);
  const [errorMsg, setErrorMsg] = useState('');
  const [bulkDeleteProgress, setBulkDeleteProgress] = useState({ completed: 0, total: 0 });
  const [bulkDeleteErrors, setBulkDeleteErrors] = useState<{ id: ID; reason: string }[]>([]);
  const [bulkDeleteSuccessCount, setBulkDeleteSuccessCount] = useState(0);

  const { plugin, id } = useParams() as { plugin: string; id: string };
  const connectionId = +id;

  const {
    token: { colorPrimary },
  } = theme.useToken();

  const dispatch = useAppDispatch();
  const connection = useAppSelector((state) => selectConnection(state, `${plugin}-${connectionId}`)) as IConnection;

  const navigate = useNavigate();

  const { ready, data } = useRefreshData(
    () => API.scope.list(plugin, connectionId, { page, pageSize, blueprints: true }),
    [version, page, pageSize],
  );

  const { name } = connection;

  const pluginConfig = useMemo(() => getPluginConfig(plugin), [plugin]);

  const [dataSource, total] = useMemo(
    () => [
      data?.scopes.map((it: any) => ({
        id: getPluginScopeId(plugin, it.scope),
        name: getPluginScopeName(plugin, it.scope) || it.scope.fullName || it.scope.name,
        projects: it.blueprints?.map((bp: any) => bp.projectName) ?? [],
        configId: it.scopeConfig?.id,
        configName: it.scopeConfig?.name,
      })) ?? [],
      data?.count ?? 0,
    ],
    [data],
  );

  const handleHideDialog = () => {
    setType(undefined);
  };

  const handleShowDeleteDialog = () => {
    setType('deleteConnection');
  };

  const handleDelete = async () => {
    const [, res] = await operator(
      async () => {
        try {
          await dispatch(removeConnection({ plugin, connectionId })).unwrap();
          return { status: 'success' };
        } catch (err: any) {
          const { status, data, message } = err;
          return {
            status: status === 409 ? 'conflict' : 'error',
            conflict: data ? [...data.projects, ...data.blueprints] : [],
            message,
          };
        }
      },
      {
        setOperating,
        hideToast: true,
      },
    );

    if (res.status === 'success') {
      message.success('Delete Connection Successful.');
      navigate(PATHS.CONNECTIONS());
    } else if (res.status === 'conflict') {
      setType('deleteConnectionFailed');
      setConflict(res.conflict);
      setErrorMsg(res.message);
    } else {
      message.error('Operation failed.');
      handleHideDialog();
    }
  };

  const handleShowCreateDataScopeDialog = () => {
    setType('createDataScope');
  };

  const handleCreateDataScope = () => {
    setVersion((v) => v + 1);
    handleHideDialog();
  };

  const handleShowClearDataScopeDialog = (scopeId: ID) => {
    setType('clearDataScope');
    setScopeId(scopeId);
  };

  const handleShowDeleteDataScopeDialog = (scopeId: ID) => {
    setType('deleteDataScope');
    setScopeId(scopeId);
  };

  const handleDeleteDataScope = async (onlyData: boolean) => {
    if (!scopeId) return;

    const [, res] = await operator(
      async () => {
        try {
          await API.scope.remove(plugin, connectionId, scopeId, onlyData);
          return { status: 'success' };
        } catch (err: any) {
          const { status, data } = err.response;
          return {
            status: status === 409 ? 'conflict' : 'error',
            conflict: data.data ? [...data.data.projects, ...data.data.blueprints] : [],
            message: data.message,
          };
        }
      },
      {
        setOperating,
        hideToast: true,
      },
    );

    if (res.status === 'success') {
      if (dataSource.length === 1 && page > 1) {
        setPage(page - 1);
      } else {
        setVersion((v) => v + 1);
      }
      message.success(onlyData ? 'Clear historical data successful.' : 'Delete Data Scope successful.');
      handleHideDialog();
    } else if (res.status === 'conflict') {
      setType('deleteDataScopeFailed');
      setConflict(res.conflict);
      setErrorMsg(res.message);
    } else {
      message.error('Operation failed.');
      handleHideDialog();
    }
  };

  const handleShowScopeConfigSelectDialog = (scopeIds: ID[]) => {
    setType('associateScopeConfig');
    setScopeIds(scopeIds);
  };

  const handleBulkDeleteScopes = async () => {
    setType('deleteSelectedScopes');
    setBulkDeleteProgress({ completed: 0, total: scopeIds.length });
    setBulkDeleteErrors([]);
    setBulkDeleteSuccessCount(0);
    setOperating(true);

    let completed = 0;
    let successCount = 0;
    const newErrors: { id: ID; reason: string }[] = [];

    const scopeMap = new Map(dataSource.map((s) => [s.id, s.name]));
    await Promise.all(
      scopeIds.map(async (id) => {
        try {
          await API.scope.remove(plugin, connectionId, id, false);
          successCount++;
        } catch (err: any) {
          const scopeName = scopeMap.get(String(id)) || 'Unknown';
          const message = err?.response?.data?.message || 'Unknown error';
          newErrors.push({
            id,
            reason: `${scopeName} - ${message}`,
          });
        } finally {
          completed++;
          setBulkDeleteProgress({ completed, total: scopeIds.length });
        }
      }),
    );

    setBulkDeleteErrors(newErrors);
    setBulkDeleteSuccessCount(successCount);
    setOperating(false);
    setVersion((v) => v + 1);
    setScopeIds([]);
    setPage(1);
  };

  const handleAssociateScopeConfig = async (trId: ID) => {
    const [success] = await operator(
      () =>
        Promise.all(
          scopeIds.map(async (scopeId) => {
            const scope = await API.scope.get(plugin, connectionId, scopeId);
            return API.scope.update(plugin, connectionId, scopeId, {
              ...scope,
              scopeConfigId: trId !== 'None' ? +trId : null,
            });
          }),
        ),
      {
        setOperating,
        formatMessage: () =>
          trId !== 'None' ? 'Associate scope config successful.' : 'Dis-associate scope config successful.',
      },
    );

    if (success) {
      handleHideDialog();
      setVersion(version + 1);
      message.success(
        'Scope Config(s) have been updated. If you would like to re-transform or re-collect the data in the related project(s), please go to the Project page and do so.',
      );
    }
  };

  return (
    <PageHeader
      breadcrumbs={[
        { name: 'Connections', path: PATHS.CONNECTIONS() },
        { name, path: '' },
      ]}
      extra={
        <Button type="primary" danger icon={<DeleteOutlined />} onClick={handleShowDeleteDialog}>
          Delete Connection
        </Button>
      }
    >
      <Helmet>
        <title>
          {connection.name} - {brandName}
        </title>
      </Helmet>
      <Space style={{ display: 'flex' }} direction="vertical" size={36}>
        <Space direction="vertical" size="small">
          <div>
            <span style={{ marginRight: 4 }}>Status:</span>
            <ConnectionStatus connection={connection} />
          </div>
          {pluginConfig.connection.renderConnectionMeta?.(connection, 'detail')}
        </Space>
        <div>Please note: In order to view DORA metrics, you will need to add Scope Configs.</div>
        <div>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleShowCreateDataScopeDialog}>
            Add Data Scope
          </Button>
          {plugin !== 'tapd' && pluginConfig.scopeConfig && (
            <Button
              style={{ marginLeft: 8 }}
              type="primary"
              disabled={!scopeIds.length}
              icon={<LinkOutlined />}
              onClick={() => handleShowScopeConfigSelectDialog(scopeIds)}
            >
              Associate Scope Config
            </Button>
          )}
          {dataSource.length > 0 && (
            <Button
              style={{ marginLeft: 8 }}
              danger
              type="primary"
              disabled={!scopeIds.length}
              icon={<DeleteOutlined />}
              onClick={() => setType('confirmDeleteSelectedScopes')}
            >
              Delete Data Scope
            </Button>
          )}
        </div>
        <Table
          rowKey="id"
          size="middle"
          loading={!ready}
          columns={[
            {
              title: 'Data Scope',
              dataIndex: 'name',
              key: 'name',
            },
            {
              title: 'Project',
              dataIndex: 'projects',
              key: 'projects',
              render: (projects) => (
                <>
                  {projects.length ? (
                    <ul>
                      {projects.map((it: string) => (
                        <li key={it}>
                          <Link to={PATHS.PROJECT(it)}>{it}</Link>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    '-'
                  )}
                </>
              ),
            },
            {
              title: 'Scope Config',
              key: 'scopeConfig',
              width: 400,
              render: (_, { id, name, configId, configName }) => (
                <ScopeConfig
                  plugin={plugin}
                  connectionId={connectionId}
                  scopeId={id}
                  scopeName={name}
                  scopeConfigId={configId}
                  scopeConfigName={configName}
                  onSuccess={() => setVersion(version + 1)}
                />
              ),
            },
            {
              title: '',
              dataIndex: 'id',
              key: 'id',
              align: 'center',
              width: 200,
              render: (id) => (
                <Space>
                  <IconButton
                    type="primary"
                    icon={<ClearOutlined />}
                    helptip="Clear Data Scope"
                    onClick={() => handleShowClearDataScopeDialog(id)}
                  />
                  <IconButton
                    type="primary"
                    icon={<DeleteOutlined />}
                    helptip="Delete Data Scope"
                    onClick={() => handleShowDeleteDataScopeDialog(id)}
                  />
                </Space>
              ),
            },
          ]}
          dataSource={dataSource}
          pagination={{
            current: page,
            pageSize,
            total,
            onChange: setPage,
            onShowSizeChange: (_, size) => {
              setPage(1);
              setPageSize(size);
            },
          }}
          rowSelection={{
            selectedRowKeys: scopeIds,
            onChange: (selectedRowKeys) => setScopeIds(selectedRowKeys as ID[]),
          }}
        />
      </Space>
      {type === 'deleteConnection' && (
        <Modal
          open
          width={820}
          centered
          title="Would you like to delete this Data Connection?"
          okText="Confirm"
          okButtonProps={{
            loading: operating,
          }}
          onCancel={handleHideDialog}
          onOk={handleDelete}
        >
          <Message
            content=" This operation cannot be undone. Deleting a Data Connection will delete all data that have been collected
              in this Connection."
          />
        </Modal>
      )}
      {type === 'createDataScope' && (
        <Modal
          getContainer={false}
          open
          width={820}
          centered
          style={{ width: 820 }}
          footer={null}
          title={
            <S.ModalTitle>
              <span className="icon">{pluginConfig.icon({ color: colorPrimary })}</span>
              <span className="name">Add Data Scope: {name}</span>
            </S.ModalTitle>
          }
          onCancel={handleHideDialog}
        >
          <DataScopeRemote
            plugin={plugin}
            connectionId={connectionId}
            disabledScope={dataSource}
            onCancel={handleHideDialog}
            onSubmit={handleCreateDataScope}
          />
        </Modal>
      )}
      {type === 'clearDataScope' && (
        <Modal
          open
          width={820}
          centered
          title="Would you like to clear the historical data of the selected Data Scope?"
          okText="Confirm"
          okButtonProps={{
            loading: operating,
          }}
          onCancel={handleHideDialog}
          onOk={() => handleDeleteDataScope(true)}
        >
          <Message content="This operation cannot be undone." />
        </Modal>
      )}
      {type === 'deleteDataScope' && (
        <Modal
          open
          width={820}
          centered
          title="Would you like to delete the selected Data Scope?"
          okText="Confirm"
          okButtonProps={{
            loading: operating,
          }}
          onCancel={handleHideDialog}
          onOk={() => handleDeleteDataScope(false)}
        >
          <Message
            content="This operation cannot be undone. Deleting Data Scope will delete all data that have been collected in the
              past."
          />
        </Modal>
      )}
      {type === 'associateScopeConfig' && (
        <Modal
          open
          width={960}
          centered
          footer={null}
          title={
            <S.ModalTitle>
              <span className="icon">{pluginConfig.icon({ color: colorPrimary })}</span>
              <span>Associate Scope Config</span>
            </S.ModalTitle>
          }
          onCancel={handleHideDialog}
        >
          <ScopeConfigSelect
            plugin={plugin}
            connectionId={connectionId}
            onCancel={handleHideDialog}
            onSubmit={handleAssociateScopeConfig}
          />
        </Modal>
      )}
      {type === 'deleteConnectionFailed' && (
        <Modal
          open
          width={820}
          centered
          style={{ width: 820 }}
          title="This Data Connection can not be deleted."
          cancelButtonProps={{
            style: {
              display: 'none',
            },
          }}
          onCancel={handleHideDialog}
          onOk={handleHideDialog}
        >
          {!conflict.length ? (
            <Message content={errorMsg} />
          ) : (
            <>
              <Message
                content={`This Data Connection can not be deleted because it has been used in the following projects/blueprints:`}
              />
              <ul style={{ paddingLeft: 36 }}>
                {conflict.map((it) => (
                  <li key={it} style={{ color: colorPrimary }}>
                    {it}
                  </li>
                ))}
              </ul>
            </>
          )}
        </Modal>
      )}
      {type === 'deleteDataScopeFailed' && (
        <Modal
          open
          width={820}
          centered
          title="This Data Scope can not be deleted."
          cancelButtonProps={{
            style: {
              display: 'none',
            },
          }}
          onCancel={handleHideDialog}
          onOk={handleHideDialog}
        >
          {!conflict.length ? (
            <Message content={errorMsg} />
          ) : (
            <>
              <Message content="This Data Scope can not be deleted because it has been used in the following projects/blueprints:" />
              <ul style={{ paddingLeft: 36 }}>
                {conflict.map((it) => (
                  <li key={it} style={{ color: colorPrimary }}>
                    {it}
                  </li>
                ))}
              </ul>
            </>
          )}
        </Modal>
      )}
      {type === 'confirmDeleteSelectedScopes' && (
        <Modal
          open
          width={720}
          centered
          title="Are you sure you want to delete selected Data Scopes?"
          okText="Yes, Delete"
          cancelText="Cancel"
          okButtonProps={{ danger: true }}
          onCancel={handleHideDialog}
          onOk={() => {
            handleBulkDeleteScopes();
          }}
        >
          <Message content="This operation cannot be undone. All selected scopes and their historical data will be permanently removed." />
          <div style={{ marginTop: 12 }}>
            <div>
              You are about to delete <strong>{scopeIds.length}</strong> data scopes:
            </div>
            <ul style={{ marginTop: 8, paddingLeft: 20 }}>
              {scopeIds.slice(0, 5).map((id) => {
                const scope = dataSource.find((s) => s.id === id);
                return <li key={id}>{scope?.name || `Scope ID ${id}`}</li>;
              })}
              {scopeIds.length > 5 && <li>...and {scopeIds.length - 5} more</li>}
            </ul>
          </div>
        </Modal>
      )}
      {type === 'deleteSelectedScopes' && (
        <Modal
          open
          width={820}
          centered
          title="Delete Selected Data Scopes"
          okText="Ok"
          okButtonProps={{ loading: operating }}
          cancelButtonProps={{ style: { display: 'none' } }}
          onOk={handleHideDialog}
        >
          <Message content="This will delete all selected data scopes. This operation cannot be undone. If any scope fails to delete, it will be listed below after the operation." />

          <div style={{ marginTop: 16, fontWeight: 500 }}>
            Progress: {bulkDeleteProgress.completed}/{bulkDeleteProgress.total}
          </div>

          {bulkDeleteProgress.completed === bulkDeleteProgress.total && (
            <div style={{ marginTop: 24 }}>
              <div style={{ fontWeight: 'bold', marginBottom: 8 }}>Summary:</div>
              <div style={{ marginLeft: 16 }}>
                <div>
                  Successful deletions: <strong>{bulkDeleteSuccessCount}</strong>
                </div>
                <div>
                  Failed deletions: <strong>{bulkDeleteErrors.length}</strong>
                </div>
              </div>
            </div>
          )}

          {bulkDeleteErrors.length > 0 && (
            <div style={{ marginTop: 24 }}>
              <div style={{ fontWeight: 'bold', marginBottom: 8 }}>Failed Deletions:</div>
              <ul style={{ marginLeft: 24, paddingLeft: 16, borderLeft: '3px solid red' }}>
                {bulkDeleteErrors.map(({ id, reason }) => (
                  <li key={id} style={{ marginBottom: 8 }}>
                    {reason}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </Modal>
      )}
    </PageHeader>
  );
};
