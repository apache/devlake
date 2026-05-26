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

import { useState } from 'react';
import { Button, Alert, Space } from 'antd';
import { CheckCircleOutlined, ExclamationCircleOutlined, LoadingOutlined } from '@ant-design/icons';

import API from '@/api';
import { operator } from '@/utils';

interface Props {
  plugin: string;
  connectionId?: ID;
  values: any;
  initialValues: any;
  disabled?: boolean;
}

interface TestResult {
  success: boolean;
  message: string;
  details?: {
    s3Access?: boolean;
    identityCenterAccess?: boolean;
  };
}

export const QDevConnectionTest = ({ plugin, connectionId, values, initialValues, disabled }: Props) => {
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<TestResult | null>(null);

  const handleTest = async () => {
    setTesting(true);
    setTestResult(null);

    try {
      const [success, result] = await operator(
        () => {
          if (connectionId) {
            // Test existing connection with only changed values
            return API.connection.test(plugin, connectionId, {
              authType: values.authType !== initialValues.authType ? values.authType : undefined,
              accessKeyId: values.accessKeyId !== initialValues.accessKeyId ? values.accessKeyId : undefined,
              secretAccessKey:
                values.secretAccessKey !== initialValues.secretAccessKey ? values.secretAccessKey : undefined,
              region: values.region !== initialValues.region ? values.region : undefined,
              bucket: values.bucket !== initialValues.bucket ? values.bucket : undefined,
              identityStoreId:
                values.identityStoreId !== initialValues.identityStoreId ? values.identityStoreId : undefined,
              identityStoreRegion:
                values.identityStoreRegion !== initialValues.identityStoreRegion
                  ? values.identityStoreRegion
                  : undefined,
              rateLimitPerHour:
                values.rateLimitPerHour !== initialValues.rateLimitPerHour ? values.rateLimitPerHour : undefined,
              proxy: values.proxy !== initialValues.proxy ? values.proxy : undefined,
            } as any);
          } else {
            // Test new connection with all values
            return API.connection.testOld(plugin, {
              authType: values.authType || 'access_key',
              accessKeyId: values.accessKeyId || '',
              secretAccessKey: values.secretAccessKey || '',
              region: values.region || '',
              bucket: values.bucket || '',
              identityStoreId: values.identityStoreId || '',
              identityStoreRegion: values.identityStoreRegion || '',
              rateLimitPerHour: values.rateLimitPerHour || 20000,
              proxy: values.proxy || '',
              endpoint: '', // Not used by Q Developer
              token: '', // Not used by Q Developer
            } as any);
          }
        },
        {
          setOperating: () => {}, // We handle loading state ourselves
          hideToast: true, // We show our own success/error messages
        },
      );

      if (success && result) {
        setTestResult({
          success: true,
          message: 'Connection test successful! AWS credentials and S3 access verified.',
          details: {
            s3Access: true,
            identityCenterAccess: values.identityStoreId ? true : undefined,
          },
        });
      } else {
        setTestResult({
          success: false,
          message: 'Connection test failed. Please check your configuration.',
        });
      }
    } catch (error: any) {
      let errorMessage = 'Connection test failed. Please check your configuration.';

      if (error?.response?.data?.message) {
        errorMessage = error.response.data.message;
      } else if (error?.message) {
        errorMessage = error.message;
      }

      // Provide more specific error messages based on common issues
      if (errorMessage.includes('InvalidAccessKeyId') || errorMessage.includes('SignatureDoesNotMatch')) {
        errorMessage = 'Invalid AWS credentials. Please check your Access Key ID and Secret Access Key.';
      } else if (errorMessage.includes('NoSuchBucket')) {
        errorMessage = 'S3 bucket not found. Please check the bucket name and region.';
      } else if (errorMessage.includes('AccessDenied')) {
        errorMessage = 'Access denied. Please check your AWS permissions for S3 and IAM Identity Center.';
      } else if (errorMessage.includes('InvalidBucketName')) {
        errorMessage = 'Invalid S3 bucket name. Please check the bucket name format.';
      } else if (errorMessage.includes('NoCredentialsError')) {
        errorMessage =
          'AWS credentials not found. Please provide valid Access Key ID and Secret Access Key, or ensure IAM role is properly configured.';
      }

      setTestResult({
        success: false,
        message: errorMessage,
      });
    } finally {
      setTesting(false);
    }
  };

  const getAlertType = () => {
    if (!testResult) return undefined;
    return testResult.success ? 'success' : 'error';
  };

  const getAlertIcon = () => {
    if (testing) return <LoadingOutlined />;
    if (!testResult) return undefined;
    return testResult.success ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />;
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      <Button
        type="default"
        loading={testing}
        disabled={disabled || testing}
        onClick={handleTest}
        style={{ marginTop: 16 }}
      >
        {testing ? 'Testing Connection...' : 'Test Connection'}
      </Button>

      {(testResult || testing) && (
        <Alert
          type={getAlertType()}
          icon={getAlertIcon()}
          message={testing ? 'Testing connection to AWS S3 and IAM Identity Center...' : testResult?.message}
          description={
            testResult?.success && testResult.details ? (
              <div>
                <div>✓ S3 Access: Verified</div>
                {testResult.details.identityCenterAccess && <div>✓ IAM Identity Center: Configured</div>}
                {!values.identityStoreId && (
                  <div style={{ marginTop: 8, color: 'var(--devlake-color-warning-alt)' }}>
                    ⚠️ IAM Identity Center not configured - user display names will show as user IDs
                  </div>
                )}
              </div>
            ) : undefined
          }
          showIcon
          style={{ marginTop: 8 }}
        />
      )}
    </Space>
  );
};
