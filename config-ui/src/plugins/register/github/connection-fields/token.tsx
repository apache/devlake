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
import { CloseOutlined, PlusOutlined, CheckCircleFilled, WarningFilled, CloseCircleFilled } from '@ant-design/icons';
import { Input, Button } from 'antd';

import API from '@/api';
import { Block, ExternalLink, Loading } from '@/components';
import { DOC_URL } from '@/release';

import * as S from './styled';

type TokenItem = {
  value: string;
  isValid?: boolean;
  from?: string;
  status?: 'success' | 'warning' | 'error';
};

interface Props {
  type: 'create' | 'update';
  connectionId?: ID;
  endpoint?: string;
  proxy: string;
  initialValue: string;
  value: string;
  error: string;
  setValue: (value: string) => void;
  setError: (error?: string) => void;
}

export const Token = ({
  type,
  connectionId,
  endpoint,
  proxy,
  initialValue,
  value,
  error,
  setValue,
  setError,
}: Props) => {
  const [loading, setLoading] = useState(false);
  const [tokens, setTokens] = useState<TokenItem[]>([{ value: '' }]);

  const testToken = async (token: string): Promise<TokenItem> => {
    if (!endpoint || !token) {
      return {
        value: token,
      };
    }

    try {
      const res = await API.connection.testOld('github', {
        authMethod: 'AccessToken',
        endpoint,
        proxy,
        token,
      });
      return {
        value: token,
        isValid: true,
        from: res.login,
        status: !res.success ? 'error' : res.warning ? 'warning' : 'success',
      };
    } catch {
      return {
        value: token,
        isValid: false,
        status: 'error',
      };
    }
  };

  const checkTokens = async (connectionId: ID) => {
    setLoading(true);
    const res = await API.connection.test('github', connectionId);
    setTokens(
      (res.tokens ?? []).map((it) => ({
        value: it.token,
        isValid: !!it.login,
        from: it.login,
        status: !it.success ? 'error' : it.warning ? 'warning' : 'success',
      })),
    );
    setLoading(false);
  };

  useEffect(() => {
    if (connectionId) {
      checkTokens(connectionId);
    }
  }, [connectionId]);

  useEffect(() => {
    setError(type === 'create' && !value ? 'token is required' : undefined);

    return () => {
      setError('');
    };
  }, [type, value]);

  useEffect(() => {
    setValue(tokens.map((it) => it.value).join(','));
  }, [tokens]);

  const handleCreateToken = () => setTokens([...tokens, { value: '' }]);

  const handleRemoveToken = (key: number) => setTokens(tokens.filter((_, i) => (i === key ? false : true)));

  const handleChangeToken = (key: number, value: string) =>
    setTokens(tokens.map((it, i) => (i === key ? { value } : it)));

  const handleTestToken = async (key: number) => {
    const token = tokens.find((_, i) => i === key) as TokenItem;
    if (token.isValid === undefined && token.value) {
      const res = await testToken(token.value);
      setTokens((tokens) => tokens.map((it, i) => (i === key ? res : it)));
    }
  };

  return (
    <Block
      title="Personal Access Token(s)"
      description={
        <>
          Add one or more personal token(s) for authentication from you and your organization members. Multiple tokens
          (from different GitHub accounts, NOT from one account) can help speed up the data collection process.{' '}
          <ExternalLink link={DOC_URL.PLUGIN.GITHUB.AUTH_TOKEN}>
            Learn how to create a personal access token
          </ExternalLink>
        </>
      }
      required
    >
      {loading ? (
        <Loading />
      ) : (
        tokens.map(({ value, isValid, status, from }, i) => (
          <S.Input key={i}>
            <div className="input">
              <Input.Password
                style={{ width: 386 }}
                placeholder="Token"
                value={value}
                onChange={(e) => handleChangeToken(i, e.target.value)}
                onBlur={() => handleTestToken(i)}
              />
              <Button type="text" icon={<CloseOutlined />} onClick={() => handleRemoveToken(i)} />
              <div className="info">
                {isValid === false && <span className="error">Invalid</span>}
                {isValid === true && <span className="success">Valid From: {from}</span>}
              </div>
            </div>
            {status && (
              <S.Alert>
                <h4>
                  {status === 'success' && <CheckCircleFilled style={{ color: 'var(--devlake-color-success)' }} />}
                  {status === 'warning' && <WarningFilled style={{ color: 'var(--devlake-color-warning-soft)' }} />}
                  {status === 'error' && <CloseCircleFilled style={{ color: 'var(--devlake-color-error-alt)' }} />}
                  <span style={{ marginLeft: 8 }}>Token Permissions</span>
                </h4>
                {status === 'success' && <p>All required fields are checked.</p>}
                {status === 'warning' && (
                  <p>
                    This token is able to collect public repositories. If you want to collect private repositories,
                    please check the field `repo`.
                  </p>
                )}
                {status === 'error' && (
                  <>
                    <p>Please check the field(s) `repo:status`, `repo_deployment`, `read:user`, `read:org`.</p>
                    <p>If you want to collect private repositories, please check the field `repo`.</p>
                  </>
                )}
              </S.Alert>
            )}
          </S.Input>
        ))
      )}
      <div className="action">
        <Button type="primary" size="small" icon={<PlusOutlined />} onClick={handleCreateToken}>
          Another Token
        </Button>
      </div>
    </Block>
  );
};
