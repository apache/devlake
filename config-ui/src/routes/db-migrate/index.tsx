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
import { ExclamationCircleOutlined } from '@ant-design/icons';
import { Card, Space, Flex, Button } from 'antd';
import { useNavigate } from 'react-router-dom';

import API from '@/api';
import { TipLayout } from '@/components';
import { PATHS } from '@/config';
import { operator } from '@/utils';

export const DBMigrate = () => {
  const [operating, setOperating] = useState(false);

  const navigate = useNavigate();

  const handleSubmit = async () => {
    const [success] = await operator(() => API.migrate(), {
      setOperating: setOperating,
    });

    if (success) {
      navigate(PATHS.ROOT());
    }
  };

  return (
    <TipLayout>
      <Card>
        <h2>
          <Space>
            <ExclamationCircleOutlined style={{ fontSize: 20, color: 'var(--devlake-color-warning-alt)' }} />
            <span>New Migration Scripts Detected</span>
          </Space>
        </h2>
        <p>
          If you have already started, please wait for database migrations to complete, do <strong>NOT</strong> close
          your browser at this time.
        </p>
        <p className="warning">
          Warning: Performing migration may wipe collected data for consistency and re-collecting data may be required.
        </p>
        <Flex justify="center">
          <Button type="primary" loading={operating} onClick={handleSubmit}>
            Proceed to Database Migration
          </Button>
        </Flex>
      </Card>
    </TipLayout>
  );
};
