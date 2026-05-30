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

export const Organization = ({ type, initialValues, values, setValues, setErrors }: Props) => {
  useEffect(() => {
    setValues({ organization: initialValues.organization ?? '' });
  }, [initialValues.organization]);

  const error = useMemo(() => {
    return values.organization?.trim() ? '' : 'Anthropic Organization ID is required';
  }, [values.organization]);

  useEffect(() => {
    setErrors({ organization: error });
  }, [error]);

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    setValues({ organization: e.target.value });
  };

  return (
    <Block
      title="Anthropic Organization"
      description="Enter the Anthropic organization from your Anthropic admin settings. DevLake uses it to scope Claude Code usage reporting."
      required
    >
      <Input
        style={{ width: 386 }}
        placeholder="e.g. org_123456789"
        status={error ? 'error' : ''}
        value={values.organization ?? ''}
        onChange={handleChange}
      />
      {error && <S.ErrorText>{error}</S.ErrorText>}
    </Block>
  );
};
