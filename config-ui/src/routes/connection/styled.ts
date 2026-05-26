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
  h2 {
    margin-top: 36px;
  }

  h5 {
    margin-top: 12px;
    font-weight: 400;
  }

  h4 {
    position: relative;
    margin-top: 24px;

    &::after {
      content: '';
      position: absolute;
      bottom: 0;
      left: 0;
      width: 48px;
      height: 4px;
      background-color: ${({ theme }) => theme.colors.primary};
    }
  }
  ul {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
  }

  li {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    margin-top: 24px;
    margin-right: 30px;
    padding: 20px 0;
    width: 160px;
    border-radius: 8px;
    border: 1px solid ${({ theme }) => (theme.mode === 'dark' ? theme.colors.border : 'transparent')};
    background-color: ${({ theme }) => theme.colors.bgContainer};
    box-shadow: 0px 2.4px 4.8px -0.8px rgba(0, 0, 0, 0.1), 0px 1.6px 8px rgba(0, 0, 0, 0.07);
    box-sizing: border-box;
    cursor: pointer;
    transition: all 0.2s linear;

    &:hover {
      background-color: ${({ theme }) => theme.colors.bgHover};
    }

    & > .beta {
      position: absolute;
      top: 0;
      right: 0;
      padding: 4px 8px;
      font-size: 12px;
      color: ${({ theme }) => theme.colors.textInverse};
      background-color: ${({ theme }) => theme.colors.warning};
      border-radius: 8px;
    }

    & > .logo {
      width: 60px;
      height: 60px;
      margin-bottom: 8px;

      & > svg {
        width: 100%;
        height: 100%;
      }
    }

    & > .name {
      position: relative;
      margin-bottom: 8px;
      padding-bottom: 8px;

      &::after {
        position: absolute;
        bottom: 0;
        left: 50%;
        margin-left: -44px;
        content: '';
        width: 88px;
        height: 1px;
        background-color: ${({ theme }) => theme.colors.border};
      }
    }

    & > .count {
      color: ${({ theme }) => theme.colors.textMuted};
    }
  }
`;

export const ModalTitle = styled.div`
  display: flex;
  align-items: center;

  .icon {
    display: inline-flex;
    margin-right: 8px;
    width: 24px;

    & > svg {
      width: 100%;
      height: 100%;
    }
  }
`;
