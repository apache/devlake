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
## blueprint

### Summary

Users can set pipeline plan by config-ui to create schedule jobs.
And config-ui will send blueprint request with cronConfig in crontab format.

### Cron Job

cronConfig should look like this: "M H D M WD"

M: minute

H: hour

D: day(month)

M: month

WD: day(week)

Please check cron time format in https://crontab.guru/

### API

POST /blueprints

```json
Request
{
	"name": "COLLECT 1648121282469",
	"tasks": [
		[
			{
				"plugin": "github",
				"options": {
					"repo": "lake",
					"owner": "merico-dev"
				}
			}
		]
	],
	"enable": true,
	"cronConfig": "103 13 /13 * *"
}
Response
{
	"id": 7,
	"createdAt": "2022-03-27T10:16:20.046+08:00",
	"updatedAt": "2022-03-27T10:16:20.046+08:00",
	"name": "COLLECT 1648121282469",
	"tasks": [
		[
			{
				"plugin": "github",
				"options": {
					"owner": "merico-dev",
					"repo": "lake"
				}
			}
		]
	],
	"enable": true,
	"cronConfig": "103 13 /13 * *"
}
```

GET /blueprints

```json
Response
{
	"id": 7,
	"createdAt": "2022-03-27T10:16:20.046+08:00",
	"updatedAt": "2022-03-27T10:16:20.046+08:00",
	"name": "COLLECT 1648121282469",
	"tasks": [
		[
			{
				"plugin": "github",
				"options": {
					"owner": "merico-dev",
					"repo": "lake"
				}
			}
		]
	],
	"enable": true,
	"cronConfig": "103 13 /13 * *"

}
```

GET /blueprints/:blueprintId

```json
Response
{
	"id": 7,
	"createdAt": "2022-03-27T10:16:20.046+08:00",
	"updatedAt": "2022-03-27T10:16:20.046+08:00",
	"name": "COLLECT 1648121282469",
	"tasks": [
		[
			{
				"plugin": "github",
				"options": {
					"owner": "merico-dev",
					"repo": "lake"
				}
			}
		]
	],
	"enable": true,
	"cronConfig": "103 13 /13 * *"
}
```

PATCH /blueprints/:blueprintId

```json
Request
{
	"name": "COLLECT 1648121282469",
	"tasks": [
		[
			{
				"plugin": "github",
				"options": {
					"repo": "lake",
					"owner": "merico-dev"
				}
			}
		]
	],
	"enable": true,
	"cronConfig": "103 13 /13 * *"
}
Response
{
	"id": 7,
	"createdAt": "2022-03-27T10:16:20.046+08:00",
	"updatedAt": "2022-03-27T10:16:20.046+08:00",
	"name": "COLLECT 1648121282469",
	"tasks": [
		[
			{
				"plugin": "github",
				"options": {
					"owner": "merico-dev",
					"repo": "lake"
				}
			}
		]
	],
	"enable": true,
	"cronConfig": "103 13 /13 * *"
}
```
