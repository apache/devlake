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
import { Input, Typography } from 'antd';

import { Block } from '@/components';

const { Text } = Typography;

interface Props {
  initialValue: string;
  value: string;
  error: string;
  setValue: (value: string) => void;
  setError: (error: string) => void;
}

// Default JUnit regex pattern - matches files starting with "devlake-", "e2e", or "qd-report-"
const DEFAULT_JUNIT_REGEX = '(devlake-|e2e|qd-report-)[0-9a-z-]+\\.(xml|junit)';

// Validate if a string is a valid regex
const isValidRegex = (pattern: string): boolean => {
  try {
    new RegExp(pattern);
    return true;
  } catch (e) {
    return false;
  }
};

export const JUnitRegex = ({ initialValue, value, error, setValue, setError }: Props) => {
  const [touched, setTouched] = useState(false);

  // Initialize with initialValue when editing
  useEffect(() => {
    if (initialValue && !value) {
      setValue(initialValue);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialValue]);

  // Validate on value change (only if field has been touched)
  useEffect(() => {
    // Only validate if user has interacted with the field
    if (!touched) {
      return;
    }

    // Empty is valid (will use default)
    if (!value || value.trim() === '') {
      setError('');
      return;
    }

    // Validate regex syntax
    if (!isValidRegex(value.trim())) {
      setError('Invalid regex pattern. Please check the syntax.');
    } else {
      setError('');
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, touched]);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setTouched(true);
    setValue(e.target.value);
  };

  const handleBlur = () => {
    setTouched(true);
  };

  return (
    <Block
      title="JUnit File Pattern"
      description={
        <>
          Regex pattern to match JUnit XML file names in CI artifacts. Leave empty to use the default pattern.
          <br />
          <Text type="secondary" style={{ fontSize: '12px' }}>
            Default: <code>{DEFAULT_JUNIT_REGEX}</code>
          </Text>
          <br />
          <Text type="secondary" style={{ fontSize: '12px' }}>
            Example pattern: <code>[0-9a-z_-]*junit[0-9a-z_-]*\.xml</code> matches{' '}
            <code>junit_operator.xml</code>, <code>junit_install_status.xml</code>
          </Text>
        </>
      }
      error={error}
    >
      <Input
        style={{ width: 386 }}
        placeholder={DEFAULT_JUNIT_REGEX}
        value={value}
        onChange={handleChange}
        onBlur={handleBlur}
      />
    </Block>
  );
};


