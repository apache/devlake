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

import { useEffect, useMemo } from 'react';
import { Button, Flex, Select } from 'antd';

import { Block } from '@/components';
import { buildTimezoneOptions, formatTimezoneLabel, getBrowserTimezone } from '@/utils/timezones';

interface Props {
  initialValue: string;
  value: string;
  setValue: (value: string) => void;
  setError: (value: string | undefined) => void;
  subLabel?: string;
  placeholder?: string;
  required?: boolean;
}

export const ConnectionTimezone = ({
  initialValue,
  value,
  setValue,
  setError,
  subLabel,
  placeholder,
  required = false,
}: Props) => {
  const browserTimezone = useMemo(() => getBrowserTimezone(), []);
  const options = useMemo(() => buildTimezoneOptions(value), [value]);

  useEffect(() => {
    setValue(initialValue);
  }, [initialValue]);

  useEffect(() => {
    if (required && !value?.trim()) {
      setError('Timezone is required');
      return;
    }
    setError(undefined);
  }, [value, required]);

  return (
    <Block
      title="Timezone"
      required={required}
      description={
        subLabel ??
        'Select the timezone used by your data source. Search by city, region, or UTC offset. Ticket dates in Grafana depend on this value.'
      }
    >
      <Flex vertical gap="small" style={{ width: 386 }}>
        <Select
          showSearch
          placeholder={placeholder ?? 'Search timezone…'}
          value={value || undefined}
          options={options}
          optionFilterProp="label"
          filterOption={(input, option) => {
            const query = input.toLowerCase();
            const label = String(option?.label ?? '').toLowerCase();
            const tz = String(option?.value ?? '').toLowerCase();
            return label.includes(query) || tz.includes(query);
          }}
          onChange={setValue}
        />
        {browserTimezone && browserTimezone !== value && (
          <Button type="link" style={{ alignSelf: 'flex-start', padding: 0 }} onClick={() => setValue(browserTimezone)}>
            Use browser timezone ({formatTimezoneLabel(browserTimezone)})
          </Button>
        )}
      </Flex>
    </Block>
  );
};
