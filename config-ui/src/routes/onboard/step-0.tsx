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

import { useState, useContext } from 'react';
import { useNavigate } from 'react-router-dom';
import { ExclamationCircleOutlined, CloseOutlined } from '@ant-design/icons';
import { Modal, Flex, Button } from 'antd';
import styled from 'styled-components';

import API from '@/api';
import { Logo } from '@/components';
import { PATHS } from '@/config';
import { operator } from '@/utils';

import { Context } from './context';

const Wrapper = styled.div`
  .logo {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 200px;
  }

  h1 {
    margin-bottom: 24px;
    font-size: 64px;
    font-weight: 400;

    & > span {
      color: #e8471c;
    }
  }

  h4 {
    margin-bottom: 70px;
    font-size: 16px;
    font-weight: 400;
  }

  .action {
    margin: 0 auto;
    width: 280px;
  }
`;

interface Props {
  logo?: React.ReactNode;
  title?: React.ReactNode;
}

export const Step0 = ({ logo = <Logo direction="horizontal" />, title = 'DevLake' }: Props) => {
  const [operating, setOperating] = useState(false);

  const navigate = useNavigate();

  const [modal, contextHolder] = Modal.useModal();

  const { step, records, done, projectName, plugin, setStep } = useContext(Context);

  const handleClose = () => {
    modal.confirm({
      width: 820,
      title: 'Are you sure to exit the onboarding session?',
      content: 'You can get back to this session via the card on top of the Projects page.',
      icon: <ExclamationCircleOutlined />,
      okText: 'Confirm',
      onOk: async () => {
        const [success] = await operator(
          () => API.store.set('onboard', { step: 0, records, done, projectName, plugin }),
          {
            setOperating,
            hideToast: true,
          },
        );

        if (success) {
          navigate(PATHS.ROOT());
        }
      },
    });
  };

  const handleSubmit = async () => {
    const [success] = await operator(
      async () => API.store.set('onboard', { step: 1, records, done, projectName, plugin }),
      {
        setOperating,
        hideToast: true,
      },
    );

    if (success) {
      setStep(step + 1);
    }
  };

  return (
    <Wrapper>
      {contextHolder}
      <div className="logo">
        {logo}
        <CloseOutlined style={{ fontSize: 18, color: 'var(--devlake-color-text-subdued)', cursor: 'pointer' }} onClick={handleClose} />
      </div>
      <Flex vertical justify="center" align="center">
        <h1>
          Welcome to <span>{title}</span>
        </h1>
        <h4>With just a few clicks, you can integrate your initial DevOps tool and observe engineering metrics.</h4>
        <div className="action">
          <Button block size="large" type="primary" loading={operating} onClick={handleSubmit}>
            Connect to your first repository
          </Button>
        </div>
      </Flex>
    </Wrapper>
  );
};
