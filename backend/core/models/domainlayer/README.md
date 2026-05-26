<!--
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->
# Why

We have Jira/TAPD for **Project Management**, we have Github/Gitlab/BitBucket for **Code Hosting**, that is, we have
multiple _Platforms_ for a certain type of problem. So, how can we calculate metrics across different _Platforms_?

For example, some users may use Jira as their **Project Management** platform, the others might opt for TAPD, if we
were to implement a Requirement Count metrics for all users, should we implement 2 charts for Jira and TAPD
independently? It's too impractical to begin with.

# How

Domain Layer is designed to solve the problem by offering a set of Platform Independent Entities, Devlake divides all
platforms into three categories: Project Management / Code Hosting / Devops, by abstracting common properties from
different platforms, we can define a set of Domain Entities for each category.

The following rules make sure Domain Layer Entities serve its purpose

1. Every platform specific entity can be mapped (or split) to one (or more) Domain Layer Entity
2. Every Domain Layer Entity contains enough information for metrics calculation
3. Domain Layer Entity should contains some sort of pointer to its origin record, and all entities should share a same
   schema

# What

## Domain Layer Entity

- Each **Domain Entity** has a `Id` with type `string` describe its origin record in format
  `<Plugin>:<Entity>:<PK0>:<PK1>`, because:
    1. Different platforms might choice different types as their Primary Key, i.e. `AutoIncremental Integer` or `uuid`
    2. Platform might or might not use `composite primary keys`
    3. Primary key might overlay between entities, and multiple entities most likely will be combined into one table
    4. Different plugins might use same entity name, even they can not co-exists at the same time, so plugin name must
       be
       included for distinction
    5. This format is deterministic, each of every entity can be converted independently in parallel, and data could be
       rebuilt arbitrary time with same output, which mean you can truncate any table at any time, and data integrity
       will be restored on next run. (this is not possible for `AutoIncremental Integer` or `uuid`)
- Each **Domain Entity** must contains enough fields needed for all metric calculations

## Data Conversion

- Read data from platform specific table, convert and store record into one(or multiple) domain table(s)
- Generate its own `Id` accordingly
- Generate foreign key accordingly
- Fields conversion

Sample code:

```go

type Issue struct {
    Id       string  `gorm:"primaryKey"`
    BoardId  string  `gorm:"index"`
    ...
}

issue := Issue {
    Id:         "jira:JiraIssues:1:10",
    BoardId:    "jira:JiraBoard:1:10"
    ...
}

```
