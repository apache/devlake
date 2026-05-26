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

import { useNavigate } from 'react-router-dom';
import { ExclamationCircleOutlined } from '@ant-design/icons';
import { Card, Space, Flex, Button } from 'antd';

import { TipLayout } from '@/components';

export const NotFound = () => {
  const navigate = useNavigate();

  return (
    <TipLayout>
      <Card>
        <h2>
          <Space>
            <ExclamationCircleOutlined style={{ fontSize: 20, color: 'var(--devlake-color-warning-alt)' }} />
            <span>404 Not Found</span>
          </Space>
        </h2>
        <p>This is an invalid address.</p>
        <Flex justify="center">
          <Button type="primary" onClick={() => navigate('/')}>
            Go HomePage
          </Button>
        </Flex>
      </Card>
    </TipLayout>
  );
};
