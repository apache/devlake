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

import { useEffect, useMemo, type ChangeEvent } from 'react';
import { Input } from 'antd';

import { Block } from '@/components';

import * as S from './styled';

interface Props {
  type: 'create' | 'update';
  initialValues: any;
  values: any;
  setValues: (value: any) => void;
  setErrors: (value: any) => void;
}

export const Token = ({ type, initialValues, values, setValues, setErrors }: Props) => {
  useEffect(() => {
    setValues({ token: type === 'create' ? initialValues.token ?? '' : undefined });
  }, [type, initialValues.token]);

  const error = useMemo(() => {
    if (type === 'update') return '';
    return values.token?.trim() ? '' : 'API Key is required';
  }, [type, values.token]);

  useEffect(() => {
    setErrors({ token: error });
  }, [error]);

  return (
    <Block
      title="API Key"
      description="Create a Team or Enterprise Admin key in the Cursor dashboard under API Keys (admin:* scope). Enterprise keys unlock AI code tracking metrics. User API keys from Settings → Integrations do not work."
      required
    >
      <Input.Password
        style={{ width: 386 }}
        placeholder={type === 'update' ? '********' : 'Your Admin API Key'}
        value={values.token}
        onChange={(e: ChangeEvent<HTMLInputElement>) => setValues({ token: e.target.value })}
      />
      {error && <S.ErrorText>{error}</S.ErrorText>}
    </Block>
  );
};
