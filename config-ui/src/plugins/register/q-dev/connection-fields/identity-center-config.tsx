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

import { ChangeEvent, useEffect, useMemo, useRef } from 'react';
import { Input } from 'antd';

import { Block } from '@/components';

interface Props {
  initialValues: any;
  values: any;
  setValues: (values: any) => void;
  setErrors: (errors: any) => void;
}

const STORE_ID_PATTERN = /^d-[a-z0-9]{10}$/;
const REGION_PATTERN = /^[a-z]{2}-[a-z]+-\d$/;

export const IdentityCenterConfig = ({ initialValues, values, setValues, setErrors }: Props) => {
  const identityStoreId = values.identityStoreId ?? '';
  const identityStoreRegion = values.identityStoreRegion ?? '';

  useEffect(() => {
    if (values.identityStoreId === undefined) {
      setValues({ identityStoreId: initialValues.identityStoreId ?? '' });
    }
  }, [initialValues.identityStoreId, values.identityStoreId, setValues]);

  useEffect(() => {
    if (values.identityStoreRegion === undefined) {
      setValues({ identityStoreRegion: initialValues.identityStoreRegion ?? '' });
    }
  }, [initialValues.identityStoreRegion, values.identityStoreRegion, setValues]);

  const storeIdError = useMemo(() => {
    if (!identityStoreId) {
      return '';
    }
    if (!STORE_ID_PATTERN.test(identityStoreId)) {
      return 'Expected format d-xxxxxxxxxx (lowercase letters and digits).';
    }
    return '';
  }, [identityStoreId]);

  const regionError = useMemo(() => {
    if (!identityStoreRegion) {
      return identityStoreId ? 'Identity Center region is required when providing an Identity Store ID.' : '';
    }
    if (!REGION_PATTERN.test(identityStoreRegion)) {
      return 'Region should look like us-east-1.';
    }
    return '';
  }, [identityStoreRegion, identityStoreId]);

  const storeIdErrorRef = useRef<string>();
  const regionErrorRef = useRef<string>();

  useEffect(() => {
    if (storeIdErrorRef.current !== storeIdError) {
      storeIdErrorRef.current = storeIdError;
      setErrors({ identityStoreId: storeIdError });
    }
  }, [storeIdError, setErrors]);

  useEffect(() => {
    if (regionErrorRef.current !== regionError) {
      regionErrorRef.current = regionError;
      setErrors({ identityStoreRegion: regionError });
    }
  }, [regionError, setErrors]);

  const handleStoreIdChange = (e: ChangeEvent<HTMLInputElement>) => {
    setValues({ identityStoreId: e.target.value.trim() });
  };

  const handleRegionChange = (e: ChangeEvent<HTMLInputElement>) => {
    setValues({ identityStoreRegion: e.target.value.trim() });
  };

  return (
    <>
      <Block
        title="IAM Identity Store ID"
        description="Optional. Provide if you want DevLake to resolve user display names (format d-xxxxxxxxxx)."
      >
        <Input
          style={{ width: 386 }}
          placeholder="d-1234567890"
          value={identityStoreId}
          onChange={handleStoreIdChange}
          status={storeIdError ? 'error' : ''}
        />
        {storeIdError && <div style={{ marginTop: 4, color: 'var(--devlake-color-error)' }}>{storeIdError}</div>}
      </Block>

      <Block
        title="IAM Identity Center Region"
        description="Optional. Required only when Identity Store ID is provided (e.g. us-east-1)."
      >
        <Input
          style={{ width: 386 }}
          placeholder="us-east-1"
          value={identityStoreRegion}
          onChange={handleRegionChange}
          status={regionError ? 'error' : ''}
        />
        {regionError && <div style={{ marginTop: 4, color: 'var(--devlake-color-error)' }}>{regionError}</div>}
      </Block>
    </>
  );
};
