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
  padding: 16px 24px;
  background-color: ${({ theme }) => theme.colors.bgElevated};

  .title {
    display: flex;
    align-items: center;
    justify-content: space-between;

    h3 {
      margin: 0;
      padding: 0;
    }

    span {
      font-size: 10px;
      color: ${({ theme }) => theme.colors.textFaint};
    }
  }

  p.description {
    margin: 8px 0 16px;
    font-size: 12px;
  }

  .content {
    padding: 10px;
    max-height: 600;
    background-color: ${({ theme }) => theme.colors.bgContainer};
    border-radius: 4px;
    box-shadow: 1px 1px 3px 0px rgb(0 0 0 / 20%) inset;
    overflow-y: auto;

    pre {
      margin: 0;
      font-size: 10px;
    }
  }
`;
