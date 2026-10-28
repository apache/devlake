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
import { Select } from 'antd';

import { Block } from '@/components';

interface Props {
  initialValue: string;
  value: string;
  error: string;
  setValue: (value: string) => void;
  setError: (error: string) => void;
}

const CI_TOOL_OPTIONS = [
  { 
    label: 'Openshift CI', 
    value: 'Openshift CI',
    description: 'Pulls test metadata from GCS (Google Cloud Storage) based on GitHub organization scopes'
  },
  { 
    label: 'Tekton CI', 
    value: 'Tekton CI',
    description: 'Pulls data from OCI artifacts in Quay.io based on the organization you define'
  },
];

export const CIToolSelect = ({ initialValue, value, setValue, setError }: Props) => {
  // Initialize with initialValue when editing
  useEffect(() => {
    if (initialValue && !value) {
      setValue(initialValue);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialValue]);

  // Validate on value change
  useEffect(() => {
    if (!value || value.trim() === '') {
      setError('CI tool is required');
    } else {
      setError('');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  const handleChange = (selectedValue: string) => {
    setValue(selectedValue);
  };

  // Get description for selected CI tool
  const selectedOption = CI_TOOL_OPTIONS.find(opt => opt.value === value);
  const description = selectedOption 
    ? selectedOption.description 
    : 'Select the CI tool type for this connection';

  return (
    <Block
      title="CI Tool"
      description={description}
      required
    >
      <Select
        style={{ width: 386 }}
        placeholder="Select CI tool..."
        value={value || undefined}
        onChange={handleChange}
        options={CI_TOOL_OPTIONS.map(opt => ({
          label: opt.label,
          value: opt.value,
        }))}
      />
    </Block>
  );
};

