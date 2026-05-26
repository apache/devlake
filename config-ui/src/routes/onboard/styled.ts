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
  width: 100%;
  height: 100vh;
  background-color: ${({ theme }) => theme.colors.bgLayout};
`;

export const Inner = styled.div`
  margin: 0 auto;
  padding: 36px 0;
  width: 1200px;
`;

export const Header = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
`;

export const Content = styled.div`
  margin: 0 auto;
  width: 860px;
`;

export const Step = styled.ul`
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 100px;
  margin-bottom: 50px;
`;

export const StepItem = styled.li<{ $activated: boolean; $activatedColor: string }>`
  display: flex;
  align-items: center;
  position: relative;

  span:first-child {
    display: flex;
    align-items: center;
    justify-content: center;
    margin-right: 8px;
    width: 32px;
    height: 32px;
    color: ${({ theme }) => theme.colors.textDisabled};
    border: 1px solid ${({ theme }) => theme.colors.borderStep};
    border-radius: 50%;

    ${({ $activated, $activatedColor, theme }) =>
      $activated
        ? `
          color: ${theme.colors.textInverse};
          background-color: ${$activatedColor};
          border: none;
          `
        : ''}
  }

  span:last-child {
    ${({ $activated }) =>
      $activated
        ? `
    font-size: 24px;
    font-weight: 600;`
        : ''}
  }

  &::before {
    content: '';
    position: absolute;
    top: 18px;
    left: -150px;
    width: 100px;
    height: 1px;
    background-color: ${({ theme }) => theme.colors.borderStep};
  }

  &:first-child::before {
    display: none;
  }
`;

export const StepContent = styled.div`
  display: flex;
  height: 450px;
  background-color: ${({ theme }) => theme.colors.bgContainer};
  box-shadow: 0px 2.4px 4.8px -0.8px rgba(0, 0, 0, 0.1), 0px 1.6px 8px 0px rgba(0, 0, 0, 0.07);

  .content {
    flex: 0 0 540px;
    padding: 24px;
  }

  .qa {
    flex: auto;
    margin: 12px 0;
    padding: 0 24px;
    font-size: 14px;
    border-left: 1px solid ${({ theme }) => theme.colors.borderSubtle};
    overflow-y: auto;

    img {
      width: 100%;
    }

    h5 {
      margin-top: 16px;
      margin-bottom: 16px;
    }

    ul {
      padding-left: 1em;
      list-style: disc;
    }

    ol {
      padding-left: 1.5em;
    }

    li {
      font-size: 12px;
      line-height: 20px;
    }

    p {
      color: ${({ theme }) => theme.colors.textBody};
    }

    code {
      padding: 2px;
      font-size: 12px;
      font-family: Menlo;
      line-height: 20px;
      border-radius: 3px;
      border: 1px solid ${({ theme }) => theme.colors.borderSubtle};
      background: ${({ theme }) => theme.colors.bgCode};
    }
  }
`;
