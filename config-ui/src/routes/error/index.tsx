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

import { useRouteError, useNavigate } from 'react-router-dom';
import { CloseCircleOutlined } from '@ant-design/icons';
import { Card, Space, Flex, Button } from 'antd';

import { TipLayout } from '@/components';
import { PATHS } from '@/config';

export const Error = () => {
  const error = useRouteError() as Error;

  const navigate = useNavigate();
  const handleResetError = () => navigate(PATHS.ROOT());

  return (
    <TipLayout>
      <Card>
        <Space>
          <CloseCircleOutlined style={{ fontSize: 20, color: 'var(--devlake-color-error)' }} />
          <h2 style={{ color: 'var(--devlake-color-error)' }}>{error.toString() || 'Unknown Error'}</h2>
        </Space>
        <p>
          Please try again, if the problem persists include the above error message when filing a bug report on{' '}
          <strong>GitHub</strong>. You can also message us on <strong>Slack</strong> to engage with community members
          for solutions to common issues.
        </p>
        <Flex justify="center">
          <Space>
            <Button type="primary" onClick={handleResetError}>
              Continue
            </Button>
            <Button
              onClick={() =>
                window.open('https://github.com/apache/incubator-devlake', '_blank', 'noopener,noreferrer')
              }
            >
              Visit GitHub
            </Button>
          </Space>
        </Flex>
      </Card>
    </TipLayout>
  );
};
