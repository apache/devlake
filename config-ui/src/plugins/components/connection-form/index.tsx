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
import { isEqual, pick } from 'lodash';
import { Flex, Alert, Button } from 'antd';

import API from '@/api';
import { useAppDispatch, useAppSelector } from '@/hooks';
import { ExternalLink } from '@/components';
import { addConnection, updateConnection } from '@/features';
import { selectConnection } from '@/features/connections';
import { getPluginConfig } from '@/plugins';
import { ICustomHeader } from '@/types';
import { operator } from '@/utils';

import { Form } from './fields';

const sanitizeCustomHeaders = (headers?: Array<{ key?: string; value?: string }>): ICustomHeader[] | undefined => {
  if (!headers) {
    return headers;
  }

  return headers
    .filter((header) => header.key?.trim() || header.value?.trim())
    .map((header) => ({
      key: header.key?.trim() ?? '',
      value: header.value ?? '',
    }));
};

const buildUpdateTestPayload = (connection: any, values: any, customHeaders: ICustomHeader[] | undefined) => ({
  endpoint: isEqual(connection?.endpoint, values.endpoint) ? undefined : values.endpoint,
  authMethod: isEqual(connection?.authMethod, values.authMethod) ? undefined : values.authMethod,
  username: isEqual(connection?.username, values.username) ? undefined : values.username,
  password: isEqual(connection?.password, values.password) ? undefined : values.password,
  token: isEqual(connection?.token, values.token) ? undefined : values.token,
  appId: isEqual(connection?.appId, values.appId) ? undefined : values.appId,
  secretKey: isEqual(connection?.secretKey, values.secretKey) ? undefined : values.secretKey,
  proxy: isEqual(connection?.proxy, values.proxy) ? undefined : values.proxy,
  rateLimitPerHour: isEqual(connection?.rateLimitPerHour, values.rateLimitPerHour)
    ? undefined
    : values.rateLimitPerHour,
  dbUrl: isEqual(connection?.dbUrl, values.dbUrl) ? undefined : values.dbUrl,
  companyId: isEqual(connection?.companyId, values.companyId) ? undefined : values.companyId,
  organization: isEqual(connection?.organization, values.organization) ? undefined : values.organization,
  customHeaders: isEqual(connection?.customHeaders, customHeaders) ? undefined : customHeaders,
});

const buildCreateTestPayload = (initialValues: any, values: any, customHeaders: ICustomHeader[] | undefined) =>
  pick({ ...initialValues, ...values, customHeaders }, [
    'name',
    'endpoint',
    'token',
    'username',
    'password',
    'proxy',
    'authMethod',
    'appId',
    'secretKey',
    'accessKeyId',
    'secretAccessKey',
    'region',
    'bucket',
    'identityStoreId',
    'identityStoreRegion',
    'rateLimitPerHour',
    'tenantId',
    'tenantType',
    'dbUrl',
    'companyId',
    'organization',
    'customHeaders',
  ]);

interface Props {
  plugin: string;
  connectionId?: ID;
  onSuccess?: (id: ID) => void;
}

export const ConnectionForm = ({ plugin, connectionId, onSuccess }: Props) => {
  const [type, setType] = useState<'create' | 'update'>('create');
  const [values, setValues] = useState<any>({});
  const [errors, setErrors] = useState<Record<string, any>>({});
  const [operating, setOperating] = useState(false);

  const dispatch = useAppDispatch();
  const connection = useAppSelector((state) => selectConnection(state, `${plugin}-${connectionId}`));

  useEffect(() => {
    setType(connectionId ? 'update' : 'create');
  }, [connectionId]);

  const {
    name,
    connection: { docLink, fields, initialValues },
  } = getPluginConfig(plugin);

  const disabled = useMemo(() => {
    return Object.values(errors).some(Boolean);
  }, [errors]);

  const sanitizedCustomHeaders = useMemo(() => sanitizeCustomHeaders(values.customHeaders), [values.customHeaders]);

  const handleTest = async () => {
    const isUpdate = type === 'update' && !!connectionId;
    await operator(
      () =>
        isUpdate
          ? API.connection.test(
              plugin,
              connectionId,
              buildUpdateTestPayload(connection, values, sanitizedCustomHeaders),
            )
          : API.connection.testOld(plugin, buildCreateTestPayload(initialValues, values, sanitizedCustomHeaders)),
      {
        setOperating,
        formatMessage: () => 'Test Connection Successfully.',
      },
    );
  };

  const handleSave = async () => {
    const isCreate = !connectionId;
    const [success, res] = await operator(
      () =>
        isCreate
          ? dispatch(addConnection({ plugin, ...values, customHeaders: sanitizedCustomHeaders })).unwrap()
          : dispatch(
              updateConnection({ plugin, connectionId, ...values, customHeaders: sanitizedCustomHeaders }),
            ).unwrap(),
      {
        setOperating,
        formatMessage: () => (isCreate ? 'Create a New Connection Successful.' : 'Update Connection Successful.'),
      },
    );

    if (success) {
      onSuccess?.(res.id);
    }
  };

  return (
    <Flex vertical gap="small">
      <Alert
        message={
          <>
            {' '}
            If you run into any problems while creating a new connection for {name},{' '}
            <ExternalLink link={docLink}>check out this doc</ExternalLink>.
          </>
        }
      />
      <Form
        type={type}
        name={name}
        fields={fields}
        initialValues={connection ? { ...initialValues, ...connection } : initialValues}
        values={values}
        errors={errors}
        setValues={setValues}
        setErrors={setErrors}
      />
      <Flex justify="flex-end" gap="small">
        <Button loading={operating} disabled={disabled} onClick={handleTest}>
          Test Connection
        </Button>
        <Button type="primary" loading={operating} disabled={disabled} onClick={handleSave}>
          Save Connection
        </Button>
      </Flex>
    </Flex>
  );
};
