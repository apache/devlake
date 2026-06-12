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
import { Input, Radio } from 'antd';

import { Block } from '@/components';

interface Props {
  type: 'create' | 'update';
  initialValues: any;
  values: any;
  setValues: (values: any) => void;
  setErrors: (errors: any) => void;
}

const ACCESS_KEY_PATTERN = /^[A-Z0-9]{16,32}$/;
const REGION_PATTERN = /^[a-z]{2}-[a-z]+-\d$/;

const syncError = (
  key: string,
  error: string,
  setErrors: (errors: any) => void,
  ref: React.MutableRefObject<string | undefined>,
) => {
  if (ref.current !== error) {
    ref.current = error;
    setErrors({ [key]: error });
  }
};

export const AwsCredentials = ({ type, initialValues, values, setValues, setErrors }: Props) => {
  const isUpdate = type === 'update';

  const authType = values.authType ?? 'access_key';
  const accessKeyId = values.accessKeyId ?? '';
  const secretAccessKey = values.secretAccessKey ?? '';
  const region = values.region ?? '';

  const isAccessKeyAuth = authType === 'access_key';

  useEffect(() => {
    if (values.authType === undefined) {
      setValues({ authType: initialValues.authType ?? 'access_key' });
    }
  }, [initialValues.authType, values.authType, setValues]);

  useEffect(() => {
    if (values.accessKeyId === undefined) {
      setValues({ accessKeyId: initialValues.accessKeyId ?? '' });
    }
  }, [initialValues.accessKeyId, values.accessKeyId, setValues]);

  useEffect(() => {
    if (values.secretAccessKey === undefined) {
      setValues({ secretAccessKey: type === 'create' ? initialValues.secretAccessKey ?? '' : '' });
    }
  }, [type, initialValues.secretAccessKey, values.secretAccessKey, setValues]);

  useEffect(() => {
    if (values.region === undefined) {
      setValues({ region: initialValues.region ?? 'us-east-1' });
    }
  }, [initialValues.region, values.region, setValues]);

  const accessKeyError = useMemo(() => {
    if (!isAccessKeyAuth) return ''; // Not required for IAM role auth
    if (!accessKeyId) {
      return isUpdate ? '' : 'AWS Access Key ID is required';
    }
    if (!ACCESS_KEY_PATTERN.test(accessKeyId)) {
      return 'AWS Access Key ID must contain 16-32 uppercase letters or digits';
    }
    return '';
  }, [accessKeyId, isUpdate, isAccessKeyAuth]);

  const secretKeyError = useMemo(() => {
    if (!isAccessKeyAuth) return ''; // Not required for IAM role auth
    if (!secretAccessKey) {
      return isUpdate ? '' : 'AWS Secret Access Key is required';
    }
    if (secretAccessKey && secretAccessKey.length < 40) {
      return 'AWS Secret Access Key looks too short';
    }
    return '';
  }, [secretAccessKey, isUpdate, isAccessKeyAuth]);

  const regionError = useMemo(() => {
    if (!region) {
      return 'AWS Region is required';
    }
    if (!REGION_PATTERN.test(region)) {
      return 'AWS Region should look like us-east-1';
    }
    return '';
  }, [region]);

  const accessKeyErrorRef = useRef<string>();
  const secretKeyErrorRef = useRef<string>();
  const regionErrorRef = useRef<string>();

  useEffect(() => {
    syncError('accessKeyId', accessKeyError, setErrors, accessKeyErrorRef);
  }, [accessKeyError, setErrors]);

  useEffect(() => {
    syncError('secretAccessKey', secretKeyError, setErrors, secretKeyErrorRef);
  }, [secretKeyError, setErrors]);

  useEffect(() => {
    syncError('region', regionError, setErrors, regionErrorRef);
  }, [regionError, setErrors]);

  const handleAccessKeyChange = (e: ChangeEvent<HTMLInputElement>) => {
    setValues({ accessKeyId: e.target.value.trim() });
  };

  const handleSecretKeyChange = (e: ChangeEvent<HTMLInputElement>) => {
    setValues({ secretAccessKey: e.target.value.trim() });
  };

  const handleRegionChange = (e: ChangeEvent<HTMLInputElement>) => {
    setValues({ region: e.target.value.trim() });
  };

  const handleAuthTypeChange = (e: any) => {
    const newAuthType = e.target.value;
    setValues({ authType: newAuthType });

    // Clear access key fields when switching to IAM role
    if (newAuthType === 'iam_role') {
      setValues({
        authType: newAuthType,
        accessKeyId: '',
        secretAccessKey: '',
      });
    }
  };

  return (
    <>
      <Block title="Authentication Type" description="Choose how to authenticate with AWS" required>
        <Radio.Group value={authType} onChange={handleAuthTypeChange}>
          <Radio value="access_key">Access Key & Secret</Radio>
          <Radio value="iam_role">IAM Role (for EC2/ECS/Lambda)</Radio>
        </Radio.Group>
      </Block>

      {isAccessKeyAuth && (
        <>
          <Block
            title="AWS Access Key ID"
            description="Use the Access Key ID of the IAM user that can access your S3 bucket"
            required
          >
            <Input
              style={{ width: 386 }}
              placeholder="AKIAIOSFODNN7EXAMPLE"
              value={accessKeyId}
              onChange={handleAccessKeyChange}
              status={accessKeyError ? 'error' : ''}
            />
            {accessKeyError && <div style={{ marginTop: 4, color: 'var(--devlake-color-error)' }}>{accessKeyError}</div>}
          </Block>

          <Block
            title="AWS Secret Access Key"
            description="Use the Secret Access Key paired with the Access Key ID"
            required
          >
            <Input.Password
              style={{ width: 386 }}
              placeholder={isUpdate ? '********' : 'wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY'}
              value={secretAccessKey}
              onChange={handleSecretKeyChange}
              status={secretKeyError ? 'error' : ''}
            />
            {secretKeyError && <div style={{ marginTop: 4, color: 'var(--devlake-color-error)' }}>{secretKeyError}</div>}
          </Block>
        </>
      )}

      {!isAccessKeyAuth && (
        <Block
          title="IAM Role Authentication"
          description="DevLake will use the IAM role attached to the EC2 instance, ECS task, or Lambda function"
        >
          <div
            style={{
              padding: '12px',
              backgroundColor: 'var(--devlake-color-bg-elevated)',
              borderRadius: '6px',
              color: 'var(--devlake-color-text-muted)',
            }}
          >
            <p style={{ margin: 0 }}>
              Make sure the IAM role has the necessary S3 permissions to access your bucket. No additional credentials
              are required when using IAM role authentication.
            </p>
          </div>
        </Block>
      )}

      <Block title="AWS Region" description="Region of the S3 bucket, e.g. us-east-1" required>
        <Input
          style={{ width: 386 }}
          placeholder="us-east-1"
          value={region}
          onChange={handleRegionChange}
          status={regionError ? 'error' : ''}
        />
        {regionError && <div style={{ marginTop: 4, color: 'var(--devlake-color-error)' }}>{regionError}</div>}
      </Block>
    </>
  );
};
