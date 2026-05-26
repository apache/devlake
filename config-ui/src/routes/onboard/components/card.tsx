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

import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { CloseOutlined, LoadingOutlined, CheckCircleFilled, CloseCircleFilled } from '@ant-design/icons';
import { theme, Card, Flex, Progress, Space, Button, Modal } from 'antd';

import API from '@/api';
import { useRefreshData, useAutoRefresh } from '@/hooks';
import { operator } from '@/utils';

import { DashboardURLMap } from '../step-4';

interface Props {
  style?: React.CSSProperties;
}

export const OnboardCard = ({ style }: Props) => {
  const [operating, setOperating] = useState(false);
  const [version, setVersion] = useState(0);

  const navigate = useNavigate();

  const {
    token: { green5, orange5, red5 },
  } = theme.useToken();

  const [modal, contextHolder] = Modal.useModal();

  const { ready, data } = useRefreshData(() => API.store.get('onboard'), [version]);

  const record = useMemo(() => (data ? data.records.find((it: any) => it.plugin === data.plugin) : null), [data]);

  const tasksRes = useAutoRefresh(
    async () => {
      if ((data && data.done) || !record) {
        return;
      }

      return await API.pipeline.subTasks(record?.pipelineId as string);
    },
    [record],
    {
      cancel: (data) => {
        return !!(data && ['TASK_COMPLETED', 'TASK_PARTIAL', 'TASK_FAILED'].includes(data.status));
      },
    },
  );

  const status = useMemo(() => {
    if (!data || data.step !== 4) {
      return 'prepare';
    }

    if (!tasksRes.data) {
      return 'running';
    }

    switch (tasksRes.data.status) {
      case 'TASK_COMPLETED':
        return 'success';
      case 'TASK_PARTIAL':
        return 'partial';
      case 'TASK_FAILED':
        return 'failed';
      case 'TASK_RUNNING':
      default:
        return 'running';
    }
  }, [data, tasksRes]);

  const handleClose = async () => {
    modal.confirm({
      width: 600,
      title: 'Permanently close this entry?',
      content: 'You will not be able to get back to the onboarding session again.',
      okButtonProps: {
        loading: operating,
      },
      okText: 'Confirm',
      onOk: async () => {
        const [success] = await operator(() => API.store.set('onboard', { ...data, done: true }), {
          setOperating,
        });

        if (success) {
          setVersion(version + 1);
        }
      },
    });
  };

  if (!ready || !data || data.done) {
    return null;
  }

  return (
    <Card style={style}>
      <Flex style={{ paddingRight: 50 }} align="center" justify="space-between">
        <Flex align="center">
          {status === 'prepare' && (
            <Progress type="circle" size={30} format={() => `${data.step}/3`} percent={(data.step / 3) * 100} />
          )}
          {status === 'running' && <LoadingOutlined />}
          {status === 'success' && <CheckCircleFilled style={{ color: green5 }} />}
          {status === 'partial' && <CheckCircleFilled style={{ color: orange5 }} />}
          {status === 'failed' && <CloseCircleFilled style={{ color: red5 }} />}
          <div style={{ marginLeft: 16 }}>
            <h4>Onboarding Session</h4>
            {['prepare', 'running'].includes(status) && (
              <h5 style={{ fontWeight: 400 }}>
                You are not far from connecting to your first tool. Continue to finish it.
              </h5>
            )}
            {status === 'success' && (
              <h5 style={{ fontWeight: 400 }}>The data of your first tool has been collected. Please check it out.</h5>
            )}
            {status === 'partial' && (
              <h5 style={{ fontWeight: 400 }}>
                The data of your first tool has been parted collected. Please check it out.
              </h5>
            )}
            {status === 'failed' && (
              <h5 style={{ fontWeight: 400 }}>Something went wrong with the collection process.</h5>
            )}
          </div>
        </Flex>
        {status === 'prepare' && (
          <Space>
            <Button type="primary" onClick={() => navigate('/onboard')}>
              Continue
            </Button>
          </Space>
        )}
        {['running', 'failed'].includes(status) && (
          <Space>
            <Button type="primary" onClick={() => navigate('/onboard')}>
              Details
            </Button>
          </Space>
        )}
        {status === 'success' && (
          <Space>
            <Button type="primary" onClick={() => window.open(DashboardURLMap[data.plugin])}>
              Check Dashboard
            </Button>
            <Button onClick={handleClose}>Finish</Button>
          </Space>
        )}
        {status === 'partial' && (
          <Space>
            <Button type="primary" onClick={() => navigate('/onboard')}>
              Details
            </Button>
            <Button onClick={() => window.open(DashboardURLMap[data.plugin])}>Check Dashboard</Button>
          </Space>
        )}
      </Flex>
      <CloseOutlined
        style={{ position: 'absolute', top: 10, right: 20, cursor: 'pointer', fontSize: 12 }}
        onClick={handleClose}
      />
      {contextHolder}
    </Card>
  );
};
