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
# Plugin Implementation

If your favorite DevOps tool is not yet supported by DevLake, don't worry. It's not difficult to implement a DevLake plugin. In this post, we'll go through the basics of DevLake plugins and build an example plugin from scratch together.

## What is a plugin?

A DevLake plugin is a library that hooks into DevLake core at run-time.

A plugin may extend DevLake's capability in three ways:

1. Integrating with new data sources
2. Transforming/enriching existing data
3. Exporting DevLake data to other data systems

## Types of plugins

There are, as of now, support for two types of plugins:

1. __*Go plugins*__: These are the primary type of plugins used by DevLake, and require the developer to write the Go code starting from fetching (collecting) data from data sources to converting them into our normalized data models and storing them.
These are shared libraries built with Go's `plugin` package and are hooked into DevLake at runtime.
2. __*Python Plugins*__: These plugins serve the same purpose but are written fully in Python. They are conceptually the same but obviously different in terms of implementation. See [this manual](./python/README.md) for details on them.
They are **all** hooked into DevLake at runtime and are communicated with using RPC calls (based on shell calls as of now) into Python abstracted away by the framework.


## How do plugins work?

A plugin mainly consists of a collection of subtasks that can be executed by the DevLake framework. For data source plugins, a subtask may be collecting a single entity from the data source (e.g., issues from Jira).
Besides the subtasks, there are hooks that a plugin can implement to customize its initialization, migration, and more. See [here](#Plugin-package-and-code-structure) for details.

## Overview of developing a plugin in Go

In this section, we will walk you through one of our existing Go plugins with the goal that by the end you will have
an idea of the code you need to write for your own plugin. Most plugins follow the same code structure and layout, so,
as long as you understand one, you should have sufficient knowledge to tackle and figure out the others.

First, lets talk about the package structure. Each Go plugin is placed in the directory `plugins/<plugin-name>`. The plugins
are meant to be independent of each other, so no two plugin packages should reference each other. The `core` and `helpers` packages
contain the compile-time dependencies (within DevLake) that plugins may use. The `server` package contains the logic to bootstrap the
DevLake server and load the plugins at runtime. It should *not* be called by any of the plugins. The `impls` package contains implementations
of core interfaces and should be avoided to be used at compile-time as well.

Go plugins are compiled as *.so files and loaded in at runtime by the server, however they may also be compiled with the server code for **testing purposes**,
and we will show how that can be done [later](#Testing); doing so has the advantage of speeding up development for more rapid testing.

If you look at the plugins listed under `plugins/`, you may notice some familiar names, such as `github`, `jira`, `gitlab`, `jenkins`, etc.
These are "datasource" plugins; they do what you expect: they pull data from these datasources and after some processing and transformations (a.k.a. extraction and conversion), we store
them in the DevLake database. There are other types of plugins as well, such `gitextractor` and `refdiff`; these are "helper" plugins: they are meant to run
in conjunction with these datasource plugins and often perform additional post-processing. An important "helper" plugin is the `dora` plugin which is responsible
for DORA metric calculations on the collected data.

For this tutorial, we will use the [gitlab](https://github.com/apache/incubator-DevLake/tree/main/backend/plugins/gitlab) plugin as reference.

### Plugin package and code structure

The plugins generally break down into the following packages:
* `api`: This package contains the API endpoints (REST) that the plugin exposes. These endpoints expose CRUD operations on [connections](https://github.com/apache/incubator-DevLake/blob/main/backend/plugins/gitlab/api/connection.go),
[scopes](https://github.com/apache/incubator-DevLake/blob/main/backend/plugins/gitlab/api/scope.go), [scope-configs](https://github.com/apache/incubator-DevLake/blob/main/backend/plugins/gitlab/api/scope_config.go), and possibly more (such as [remote-scopes](https://github.com/apache/incubator-DevLake/blob/main/backend/plugins/gitlab/api/remote.go) to fetch raw scopes from the datasource if applicable).
Note the documentation structure of the functions in these example: that's the syntax for Go-Swagger which is responsible for their Swagger documentation generation.

   This is where we also define the logic used by the server to run the plugin based on the constraints set by a blueprint (the user's configuration); that is,
a "pipeline plan", which defines what input data to feed into the plugin, which subtasks (see `tasks` package) to execute, and which other plugins to run after this plugin's execution and the input to each of them (typically "helper" plugins).
See [this](https://github.com/apache/incubator-DevLake/blob/main/backend/plugins/gitlab/api/blueprint_v200.go) for reference.

   If plan generation returns an error during a scheduled blueprint run, DevLake records a failed pipeline with the planning error, start/end timestamps, and blueprint labels, then invokes the configured pipeline-status notification service after the record is committed. This pipeline has no tasks because planning did not complete. Fix the blueprint or connection and trigger a new run; rerunning tasks from this failed record cannot regenerate its plan. Manual triggers continue to return planning errors directly to the caller without creating a pipeline. If saving the failed record or sending its notification fails, check the lake logs for the additional error.

* `e2e`: This package contains "pseudo-integration-tests" for plugins; tests that use mocked data and test the plugin's extraction and conversion logic of that data against an actual database. The test data is formatted in CSV files structured based on the expected database table structures.
* `impl`: This package is where the plugin is actually defined, and is the "entrypoint": it is the implementation of multiple interfaces. See [this](https://github.com/apache/incubator-DevLake/tree/main/backend/plugins/gitlab/impl) for example.
  * At a minimum, a plugin must implement [PluginMeta](https://github.com/apache/incubator-DevLake/blob/main/backend/core/plugin/plugin_meta.go).
  * Bootstrap logic for the plugin (on server startup) is defined by implementing [PluginInit](https://github.com/apache/incubator-DevLake/blob/main/backend/core/plugin/plugin_init.go).Init(). This will typically initialize global variables accessible by the plugin.
  * APIs exposed by the plugin in the `api` package are also registered here by implementing [PluginApi](https://github.com/apache/incubator-DevLake/blob/main/backend/core/plugin/plugin_api.go).ApiResources(). You will nearly almost need this.
  * The plugin's core execution logic is defined by implementing the methods on [PluginTask](https://github.com/apache/incubator-DevLake/blob/main/backend/core/plugin/plugin_task.go). You will nearly almost need this.
  * The plugin's database migration logic (table definitions and their alterations) is registered via implementing [PluginMigration](https://github.com/apache/incubator-DevLake/blob/main/backend/core/plugin/plugin_migration.go).MigrationScripts(). You will nearly almost need this.
  * The plugin's Pipeline-Plan definition logic is registered by implementing [DataSourcePluginBlueprintV200](https://github.com/apache/incubator-DevLake/blob/main/backend/core/plugin/plugin_blueprint.go). You will need this if you're writing a datasource-based plugin.
  * You will need to implement [PluginSource](https://github.com/apache/incubator-DevLake/blob/main/backend/core/plugin/plugin_meta.go) if your plugin has APIs for one or more of Connections, Scopes and ScopeConfigs. If one or more is not applicable, the respective method should return `nil`.
  * You will need to implement [PluginModel](https://github.com/apache/incubator-DevLake/blob/main/backend/core/plugin/plugin_model.go) if your plugin has any database-based models (practically always the case). You will have to explicitly list out such models, which will be defined in the `models` package. We have an automated test `plugins/table_info_test.go` to
   detect if any models are missing from this list (and will fail the CICD execution).
* `models`: This is where all the database models for the plugin are used. These are called "tool" models, and their respective table-names are to be prefixed with `_tool_`. A subpackage, `migrationscripts` defines the    database DDL logic for setting and altering the tables for these models. These are called migrations, and they are applied sequentially based on their `Version()` on server startup, since the last successful migration.
   See [gitlab's migrations](https://github.com/apache/incubator-DevLake/tree/main/backend/plugins/gitlab/models/migrationscripts).
* `tasks`: This package contains all the functions needed by [PluginTask](https://github.com/apache/incubator-DevLake/blob/main/backend/core/plugin/plugin_task.go).SubTaskMetas():
These are the collectors, extractors and convertors of each datasource entity that you want to support. More will be said about these below.

### Collectors

Collectors fetch data from a data-source and store that raw data in the appropriate \_raw_ table in the DevLake database. The process of fetching is
more involved than it sounds, and is greatly influenced by the quality of the API you're interacting with. Things to think about are:
- Does the API support returning paginated results? You should always prefer that, if supported, for optimal performance.
- Does the API support incremental data collection? DevLake supports "bookmarking" fetched data based on a time-parameter on the data (stateful collectors), which can be
defined. This allows us to refresh only newer data and not have to fetch everything from scratch. But again, the API needs to honor fetching data on
such a basis.

See [this](https://github.com/apache/incubator-DevLake/blob/main/backend/plugins/gitlab/tasks/issue_collector.go) for an example of a stateful collector.
Here's [one](https://github.com/apache/incubator-DevLake/blob/main/backend/plugins/gitlab/tasks/account_collector.go) for a stateless collector. Be sure to
read the in-code documentation of the api-clients used in these examples to familiarize yourself with their technicalities, capabilities and/or limitations.


### Extractors

Extractors are called after the collectors are done. They convert the raw data from collectors to "tool" models. Tool Models are DevLake's representation of
a data-source's data. They need not have 1:1 correspondence with them as we are usually interested in a subset of the fields. This is also where the settings of the
configured `ScopeConfig` (if applicable) get applied: that is, this is where user-defined transformations get applied to the certain fields on the raw data to set
certain fields on the target tool model. [Here's](https://github.com/apache/incubator-DevLake/blob/main/backend/plugins/gitlab/tasks/issue_extractor.go) an
example of an extractor.


### Convertors

Convertors convert the tool models emitted by extractors to "domain models". Domain models are models agnostic in form to any data source. They are generic,
standardized representations of various concepts. For example, a generic "commit", or a generic "issue". With respect to DevLake's dashboard queries, we
write queries against the database tables of these domain models.
You will need to know to what domain model(s) a given tool model should map (Read about them [here](https://DevLake.apache.org/docs/next/DataModels/DevLakeDomainLayerSchema/)).
This also depends on the requirements of your plugin, of course. See [this package](https://github.com/apache/incubator-DevLake/tree/main/backend/core/models/domainlayer) for a list of all the domain models;
they are logically distributed into packages that represent different entities (e.g. Code, Devops, Ticket, etc).
See [here](https://github.com/apache/incubator-DevLake/blob/main/backend/plugins/gitlab/tasks/issue_convertor.go) for an example of a convertor.

<br>
<br>
We have covered all the components of a DevLake plugin. Happy coding!

### Testing

There are three types of test strategies:
1. Unit-tests: these are component level tests that mock dependencies. They typically exist next to the source file they're testing.
2. E2E-tests: these test plugin extractor and convertor subtasks by using faked data to simulate the result of data collection.
3. Integration-tests: these test the entire DevLake server as a whole. A [Go Client](https://github.com/apache/devlake/blob/main/backend/test/helper/api.go) ([example initialization](https://github.com/apache/devlake/blob/main/backend/test/helper/client_factory.go))
has been written to either interact with an existing DevLake server (via its APIs) or spin up an in-memory instance of DevLake for the purpose of testing (note you need to have
a separate database instance running). We have some integration tests written for some of our plugins (see [this](https://github.com/apache/devlake/tree/main/backend/test/e2e/manual)), which is the best
source to learn how to write these tests and interact with the client. Note that the in-memory DevLake instance directly compiles the
plugins you specify and in doing so saves a significant amount of time and overhead ([example](https://github.com/apache/devlake/blob/d717d2aa897742ab4789b9a44e9e5a4c1e28adcf/backend/test/e2e/manual/gitlab/gitlab_test.go#L62)).

We highly encourage you to leverage the Go client to perform quick tests as you write either framework-level or plugin code.

#### Full-scale testing using the client

You may also leverage the Go client to set up a full DevLake test environment. Start off by writing a test function with a body similar to this:
```go
func TestDevlakeServer(t *testing.T) {
	// set up any environment variable you need for the server here
	setupEnvironmentVars()
	_ = helper.ConnectLocalServer(t, &helper.LocalClientConfig{
		ServerPort:   8080,
		DbURL:        "mysql://merico:merico@127.0.0.1:3306/lake?charset=utf8mb4&parseTime=True", //DO NOT USE localhost - it breaks Python
		CreateServer: true,  // start an in-memory server
		TruncateDb:   false, // get rid of all existing data in the DevLake database before startup
		DropDb:       false, // drop all tables in the DevLake database before startup
		Plugins: []plugin.PluginMeta{ // list of plugins to make available to the server. Just create an empty instance of your intended plugin structs (conventionally declared in their respective `impl.go`)
			github.Github{},
			githubGraphql.GithubGraphql{},
			gitextractor.GitExtractor{},
			gitlab.Gitlab(""),
			jenkins.Jenkins{},
			webhook.Webhook{},
			pagerduty.PagerDuty{},
			tapd.Tapd{},
            //refdiff.RefDiff{}, // not needed to be specified - already included by default
            //dora.Dora{}, // not needed to be specified - already included by default
            //org.Org{}, // not needed to be specified - already included by default
		},
	})
	time.Sleep(math.MaxInt)
}

// example env variables to enable Python-debugging
func setupEnvironmentVars() {
    os.Setenv("USE_PYTHON_DEBUGGER", "pycharm")
    // The Debug host is your host-IP as seen by the IDE. This is usually just "localhost", but will be different if you're launching your IDE from WSL,
	// which you'd need if developing on Windows. In that case, it is the IP of the WSL vEthernet network interface (e.g. 192.168.0.1).
	// Make sure, additionally, that the firewall is not blocking network access to your IDE - inbound/outbound.
    os.Setenv("PYTHON_DEBUG_HOST", "localhost")
    os.Setenv("PYTHON_DEBUG_PORT", "32000")
}
```
Adjust the declared list of plugins and other parameters of the constructor based on your needs. This spins up a server on your host machine
on port 8080, and runs indefinitely.
Again, make sure your database instance is already running (likely on Docker). In the above, it is accessible via 127.0.0.1:3306.

The next step is to set up the config-ui container. You will want to spin up the container with the env variable `DEVLAKE_ENDPOINT: ${HOST_IP}:8080`,
where `{HOST_IP}` is the IP of the host machine as seen by the docker containers. Typically, on newer versions of Docker, adding this to the config-ui's
docker-compose config should suffice:
```yaml
    extra_hosts:
        - "host.docker.internal:host-gateway"
```
In which case the `{HOST_IP}` will be `host.docker.internal`. This however may not work on all operating systems, and you'll have to do
some research to find out how to access the host from within docker.

Once you get this done, the config-ui container will be able to talk to the in-memory DevLake server that you spun up on your machine; you may start using the UI as normal.
It is highly recommended that you use an IDE to spin up the server. It provides much more convenience, and allows for simple debugging using breakpoints.

Since some of this configuration requires writing custom code/config, it's recommended you write them to files that will be ignored by git (using .gitignore), so you
don't accidentally end up pushing them upstream.

#### Submit the code as open source code
We encourage ideas and contributions ~ Let's use migration scripts, domain layers and other discussed concepts to write normative and
platform-neutral code. More info at [here](https://DevLake.apache.org/docs/DataModels/DevLakeDomainLayerSchema) or contact us for ebullient help.
