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

import { useEffect } from 'react';
import { Input } from 'antd';

import { Block, ExternalLink } from '@/components';

interface Props {
  type: 'create' | 'update';
  initialValue: string;
  value: string;
  setValue: (value?: string) => void;
  setError: (value?: string) => void;
}

// Datadog authenticates with a pair of keys sent as headers, so neither the
// single-token nor the username/password field fits. On update the stored
// value arrives sanitized; sending it back would overwrite the real secret,
// so the field starts empty and only a typed value is submitted.
const secretField = (label: string, subLabel: React.ReactNode, requiredMessage: string) =>
  function SecretField({ type, initialValue, value, setValue, setError }: Props) {
    useEffect(() => {
      setValue(type === 'create' ? initialValue : undefined);
    }, [type, initialValue]);

    useEffect(() => {
      setError(type === 'create' && !value ? requiredMessage : undefined);
    }, [type, value]);

    return (
      <Block title={label} description={subLabel} required>
        <Input.Password
          style={{ maxWidth: 386 }}
          placeholder={type === 'update' ? 'Leave empty to keep the stored key' : 'Your key'}
          value={value}
          onChange={(e) => setValue(e.target.value)}
        />
      </Block>
    );
  };

export const ApiKey = secretField(
  'API Key',
  <>
    An organization API key, from{' '}
    <ExternalLink link="https://docs.datadoghq.com/account_management/api-app-keys/">
      Organization Settings &gt; API Keys
    </ExternalLink>
    .
  </>,
  'API Key is required',
);

export const ApplicationKey = secretField(
  'Application Key',
  <>
    An application key with the <code>incident_read</code> scope. Prefer one owned by a service account: a personal key
    is revoked when its owner is deactivated.
  </>,
  'Application Key is required',
);

// Not a credential, and genuinely optional: the API returns no incident
// permalink, so without this incidents have no clickable URL.
export function WebUrl({ initialValue, value, setValue }: Omit<Props, 'type' | 'setError'>) {
  useEffect(() => {
    setValue(initialValue);
  }, [initialValue]);

  return (
    <Block
      title="Datadog UI URL"
      description="Where your incidents live in the browser, e.g. https://app.datadoghq.com. Optional; without it, collected incidents carry no link."
    >
      <Input
        style={{ maxWidth: 386 }}
        placeholder="https://app.datadoghq.com"
        value={value}
        onChange={(e) => setValue(e.target.value)}
      />
    </Block>
  );
}
