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
import { useNavigate, useSearchParams } from 'react-router-dom';
import { ClockCircleOutlined, CloseCircleOutlined, StopOutlined } from '@ant-design/icons';
import { Card, Space, Flex, Button } from 'antd';

import API from '@/api';
import { TipLayout } from '@/components';
import { DEVLAKE_ENDPOINT } from '@/config';

const PATH_PREFIX = import.meta.env.DEVLAKE_PATH_PREFIX ?? '';

// Keep in sync with the AuthError* codes in backend/server/api/auth/auth.go.
type AuthErrorCode = 'forbidden' | 'expired' | 'idp_error' | 'server_error';

const MESSAGES: Record<AuthErrorCode, { title: string; description: string }> = {
  forbidden: {
    title: 'Access denied',
    description:
      'You signed in successfully, but your account is not authorized to use DevLake. If you think you should have access, request it or sign in with a different account.',
  },
  expired: {
    title: 'Sign-in expired',
    description: 'Your sign-in attempt expired or could not be verified. Please sign in again.',
  },
  idp_error: {
    title: 'Sign-in failed',
    description:
      'The identity provider could not complete your sign-in. Please try again. If the problem persists, contact your DevLake administrator.',
  },
  server_error: {
    title: 'Something went wrong',
    description:
      'DevLake could not complete your sign-in. Please try again. If the problem persists, contact your DevLake administrator.',
  },
};

const toCode = (raw: string | null): AuthErrorCode =>
  raw && Object.prototype.hasOwnProperty.call(MESSAGES, raw) ? (raw as AuthErrorCode) : 'server_error';

const ICONS: Record<AuthErrorCode, React.ReactNode> = {
  forbidden: <StopOutlined style={{ fontSize: 20, color: 'var(--devlake-color-error)' }} />,
  expired: <ClockCircleOutlined style={{ fontSize: 20, color: 'var(--devlake-color-warning-alt)' }} />,
  idp_error: <CloseCircleOutlined style={{ fontSize: 20, color: 'var(--devlake-color-error)' }} />,
  server_error: <CloseCircleOutlined style={{ fontSize: 20, color: 'var(--devlake-color-error)' }} />,
};

export const AuthError = () => {
  const [searchParams] = useSearchParams();
  const code = toCode(searchParams.get('code'));
  const provider = searchParams.get('provider') ?? '';
  const [accessRequestUrl, setAccessRequestUrl] = useState<string>();

  const navigate = useNavigate();

  useEffect(() => {
    if (code !== 'forbidden') return;
    API.auth
      .methods()
      .then((m) => setAccessRequestUrl(m.accessRequestUrl))
      .catch(() => setAccessRequestUrl(undefined));
  }, [code]);

  // Retrying would reuse the still-active IdP session and fail again, so a
  // rejected user can only end that session and pick another account.
  const handleSwitchAccount = () => {
    const query = provider ? `?provider=${encodeURIComponent(provider)}` : '';
    window.location.href = `${DEVLAKE_ENDPOINT}/auth/switch-account${query}`;
  };

  const { title, description } = MESSAGES[code];

  return (
    <TipLayout>
      <Card style={{ maxWidth: 560, margin: '0 auto' }}>
        <h2>
          <Space>
            {ICONS[code]}
            <span>{title}</span>
          </Space>
        </h2>
        <p>{description}</p>
        <Flex justify="center">
          <Space wrap>
            {code === 'forbidden' && accessRequestUrl && (
              <Button type="primary" href={accessRequestUrl} target="_blank" rel="noopener noreferrer">
                Request access
              </Button>
            )}
            {code === 'forbidden' && <Button onClick={handleSwitchAccount}>Sign in with a different account</Button>}
            {code !== 'forbidden' && (
              <Button type="primary" onClick={() => navigate(`${PATH_PREFIX}/login`)}>
                Back to sign in
              </Button>
            )}
          </Space>
        </Flex>
      </Card>
    </TipLayout>
  );
};
