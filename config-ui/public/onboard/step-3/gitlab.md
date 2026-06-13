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

##### Q1. Which types of GitLab data are collected?

The data collected includes: `issues`, `repos`, `commits`, `branches`, `merge requests`, `mr notes`, `pipeline runs`, `job runs`, `deployments`, `users`, etc.

For a comprehensive scope of data collection, refer to the [Supported Data Sources documentation](https://devlake.apache.org/docs/Overview/SupportedDataSources/#data-collection-scope-by-each-plugin).

##### Q2. What time range is covered in the data collection?

Data **updated** from the past 14 days is collected to expedite synchronization. The time range can be adjusted on the project details page at any point.

##### Q3. Is it possible to transform the collected data?

Yes, data transformations can be applied by setting up a Scope Config for the selected repositories afterward.

##### Q4. How often is the data synchronized?

Data synchronization occurs daily. This frequency can be modified on the project details page as needed.

##### Q5. Which dashboard will be shown after collection?

After data collection completes, the **Contributor Experience** dashboard will open, showing metrics like Issue Initial Response Time, PR Review Time, and Good First Issue recommendations — just like GitHub.