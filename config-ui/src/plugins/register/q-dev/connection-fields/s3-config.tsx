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

const BUCKET_PATTERN = /^[a-z0-9](?:[a-z0-9.-]{1,61}[a-z0-9])?$/;

export const S3Config = ({ initialValues, values, setValues, setErrors }: Props) => {
  const bucket = values.bucket ?? '';

  useEffect(() => {
    if (values.bucket === undefined) {
      setValues({ bucket: initialValues.bucket ?? '' });
    }
  }, [initialValues.bucket, values.bucket, setValues]);

  const bucketError = useMemo(() => {
    if (!bucket) {
      return 'S3 bucket name is required.';
    }
    if (!BUCKET_PATTERN.test(bucket) || bucket.length < 3 || bucket.length > 63 || bucket.includes('..')) {
      return 'Bucket names must be 3-63 characters, lowercase, numbers, dots or hyphens.';
    }
    return '';
  }, [bucket]);

  const bucketErrorRef = useRef<string>();
  useEffect(() => {
    if (bucketErrorRef.current !== bucketError) {
      bucketErrorRef.current = bucketError;
      setErrors({ bucket: bucketError });
    }
  }, [bucketError, setErrors]);

  const handleBucketChange = (e: ChangeEvent<HTMLInputElement>) => {
    setValues({ bucket: e.target.value.trim() });
  };

  return (
    <Block title="S3 Bucket" description="Name of the bucket that stores the Q Developer CSV files." required>
      <Input
        style={{ width: 386 }}
        placeholder="my-q-dev-data"
        value={bucket}
        onChange={handleBucketChange}
        status={bucketError ? 'error' : ''}
      />
      {bucketError && <div style={{ marginTop: 4, color: 'var(--devlake-color-error)' }}>{bucketError}</div>}
    </Block>
  );
};
