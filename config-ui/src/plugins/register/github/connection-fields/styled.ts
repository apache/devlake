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

export const Label = styled.label`
  font-size: 16px;
  font-weight: 600;
`;

export const LabelInfo = styled.i`
  color: ${({ theme }) => theme.colors.secondary};
`;

export const LabelDescription = styled.p`
  margin: 0;
`;

export const Endpoint = styled.div`
  p {
    margin: 10px 0;
  }
`;

export const Input = styled.div`
  margin-bottom: 8px;

  .input {
    display: flex;
    align-items: center;

    .info {
      margin-left: 4px;

      span.error {
        color: ${({ theme }) => theme.colors.error};
      }

      span.success {
        color: ${({ theme }) => theme.colors.success};
      }
    }
  }

  .warning {
    margin-top: 8px;
  }
`;

export const Alert = styled.div`
  margin-top: 8px;
  padding: 12px 20px;
  background: ${({ theme }) => theme.colors.bgLayout};
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: 4px;
`;
