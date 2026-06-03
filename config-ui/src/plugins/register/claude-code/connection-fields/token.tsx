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

  const customHeaders: Array<{ key: string; value: string }> = values.customHeaders ?? [];
  const hasValidCustomHeaders = customHeaders.some((h) => h.key?.trim() && h.value?.trim());
  const hasIncompleteCustomHeaders = !hasValidCustomHeaders && customHeaders.length > 0;

  const error = useMemo(() => {
    if (type === 'update') return '';
    if (hasIncompleteCustomHeaders)
      return 'Custom headers are present but none have both a key and a value. Please complete or remove them, or provide an Anthropic API Key.';
    if (hasValidCustomHeaders) return '';
    return values.token?.trim() ? '' : 'Anthropic API Key is required (unless custom headers are configured)';
  }, [type, values.token, hasValidCustomHeaders, hasIncompleteCustomHeaders]);

  useEffect(() => {
    setErrors({ token: error });
  }, [error]);

  return (
    <Block
      title="Anthropic API Key"
      description="Use an Anthropic API key with permission to read Claude Code usage reports for the selected organization."
      required={!hasValidCustomHeaders}
    >
      <Input.Password
        style={{ width: 386 }}
        placeholder={type === 'update' ? '********' : 'Your API Key'}
        value={values.token}
        onChange={(e: ChangeEvent<HTMLInputElement>) => setValues({ token: e.target.value })}
      />
      {error && <S.ErrorText>{error}</S.ErrorText>}
    </Block>
  );
};
