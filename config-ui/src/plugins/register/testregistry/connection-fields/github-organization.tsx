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

import { Block } from '@/components';

interface Props {
  initialValue: string;
  value: string;
  error: string;
  setValue: (value: string) => void;
  setError: (error: string) => void;
}

export const GitHubOrganization = ({ initialValue, value, error, setValue, setError }: Props) => {
  // Initialize with initialValue when editing
  useEffect(() => {
    if (initialValue && !value) {
      setValue(initialValue);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialValue]);

  // Validate on value change (only if value exists or was changed by user)
  useEffect(() => {
    // Skip validation on initial mount if value is empty (allows form to start without errors)
    if (value === '' && !initialValue) {
      return;
    }
    
    if (!value || value.trim() === '') {
      setError('GitHub organization is required');
    } else {
      // Basic validation: no spaces, only alphanumeric, hyphens, underscores
      const validPattern = /^[a-zA-Z0-9_-]+$/;
      if (!validPattern.test(value.trim())) {
        setError('Organization name can only contain letters, numbers, hyphens, and underscores');
      } else {
        setError('');
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setValue(e.target.value);
  };

  return (
    <Block
      title="GitHub Organization"
      description="Enter the GitHub organization name. Test metadata will be pulled from GCS (Google Cloud Storage) based on the data scopes (repositories) from this GitHub organization."
      required
      error={error}
    >
      <Input
        style={{ width: 386 }}
        placeholder="my-org"
        value={value}
        onChange={handleChange}
      />
    </Block>
  );
};

