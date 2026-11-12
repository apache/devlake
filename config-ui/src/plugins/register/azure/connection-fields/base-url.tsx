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
import { Input, Radio, type RadioChangeEvent } from 'antd';
import { Block } from '@/components';

interface Props {
  initialValue?: string;
  value?: string;
  onChange?: (value: string) => void;
  setError?: (error: string) => void;
}

export const BaseURL = ({ initialValue, value, onChange, setError }: Props) => {
  const [version, setVersion] = useState<'cloud' | 'server'>(value ? 'server' : 'cloud');

  useEffect(() => {
    if (initialValue !== undefined) {
      onChange?.(initialValue);
      setVersion(initialValue ? 'server' : 'cloud');
    }
  }, [initialValue]);

  useEffect(() => {
    setError?.(version === 'server' && !value?.trim() ? 'endpoint is required' : '');
  }, [version, value]);

  const handleVersionChange = (e: RadioChangeEvent) => {
    const selectedVersion = e.target.value;
    setVersion(selectedVersion);
    if (selectedVersion === 'cloud') {
      onChange?.('');
    }
  };

  return (
    <Block title="Azure DevOps Version" required>
      <Radio.Group name="azure-devops-version" value={version} onChange={handleVersionChange}>
        <Radio value="cloud">Azure DevOps Cloud</Radio>
        <Radio value="server">Azure DevOps Server (On-Premises)</Radio>
      </Radio.Group>

      {version === 'cloud' ? (
        <p style={{ margin: '8px 0 0 0', color: 'gray' }}>
          If you are using Azure DevOps Cloud, you do not need to enter the endpoint URL.
        </p>
      ) : (
        <div style={{ marginTop: 12 }}>
          <p style={{ margin: '0 0 8px 0', fontWeight: 500 }}>Endpoint URL *</p>
          <Input
            placeholder="e.g., https://your-server/tfs/DefaultCollection/"
            value={value}
            onChange={(e) => onChange?.(e.target.value)}
          />
          <p style={{ margin: '4px 0 0 0', color: 'gray', fontSize: 12 }}>
            Enter your full Azure DevOps Server base URL including the collection name.
          </p>
        </div>
      )}
    </Block>
  );
};
