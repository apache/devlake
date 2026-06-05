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

import { useEffect, useState } from 'react';
import { Card, Button, Typography, Alert, Space } from 'antd';

import API from '@/api';
import type { Methods, Provider } from '@/api/auth';
import { TipLayout } from '@/components';
import { DEVLAKE_ENDPOINT } from '@/config';

const { Title, Paragraph } = Typography;

export const Login = () => {
  const [methods, setMethods] = useState<Methods | null>(null);
  const [error, setError] = useState<string | null>(null);

  const params = new URLSearchParams(window.location.search);
  const returnUrl = params.get('return_url') || '/';

  useEffect(() => {
    API.auth
      .methods()
      .then(setMethods)
      .catch((e) => setError(e?.message ?? 'Failed to load login methods'));
  }, []);

  const startOIDC = (p: Provider) => {
    const sep = p.loginUrl.includes('?') ? '&' : '?';
    window.location.href = `${DEVLAKE_ENDPOINT}${p.loginUrl}${sep}return_url=${encodeURIComponent(returnUrl)}`;
  };

  const providers = methods?.providers ?? [];
  const apiKey = methods?.apiKey;
  const noProviders = providers.length === 0 && !apiKey?.enabled;

  return (
    <TipLayout>
      <Card style={{ maxWidth: 480, margin: '0 auto' }}>
        <Title level={3} style={{ textAlign: 'center' }}>
          Sign in to DevLake
        </Title>
        {error && <Alert type="error" message={error} style={{ marginBottom: 16 }} />}
        {providers.length > 0 && (
          <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
            {providers.map((p) => (
              <Button key={p.name} type="primary" size="large" block onClick={() => startOIDC(p)}>
                Sign in with {p.displayName}
              </Button>
            ))}
          </Space>
        )}
        {providers.length === 0 && apiKey?.enabled && (
          <Paragraph type="secondary">
            Single Sign-On is not configured. Use an API key (Authorization: Bearer ...) to access /rest endpoints, or
            ask your administrator to enable OIDC.
          </Paragraph>
        )}
        {noProviders && methods !== null && (
          <Alert
            type="warning"
            message="No login providers are enabled."
            description="Set AUTH_ENABLED=true and configure OIDC_PROVIDERS + per-provider env vars, or sign in upstream of DevLake."
          />
        )}
      </Card>
    </TipLayout>
  );
};
