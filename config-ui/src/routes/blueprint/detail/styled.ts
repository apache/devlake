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

import styled from 'styled-components';

export const Wrapper = styled.div`
  padding-bottom: 24px;
`;

export const DialogBody = styled.div`
  display: flex;
  align-items: center;
`;

export const ConfigurationPanel = styled.div`
  .block + .block {
    margin-top: 36px;
  }

  h3 {
    margin-bottom: 16px;
  }

  .btns {
    margin-top: 16px;
    text-align: right;
  }
`;

export const ConnectionList = styled.ul`
  display: flex;
  align-items: center;
`;

export const ConnectionItem = styled.li`
  margin-right: 24px;
  padding: 12px 16px;
  width: 280px;
  background: ${({ theme }) => theme.colors.bgContainer};
  box-shadow: 0px 2.4px 4.8px -0.8px rgba(0, 0, 0, 0.1), 0px 1.6px 8px rgba(0, 0, 0, 0.07);
  border-radius: 4px;

  &:last-child {
    margin-right: 0;
  }

  .count {
    margin: 24px 0;
  }
`;
