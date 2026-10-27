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
# Pydevlake

Pydevlake is a framework for writing plugins for [DevLake](https://devlake.apache.org/) in Python. The framework source code
can be found in [here](./pydevlake).


# How to create a new plugin

## Create the plugin project


Make sure you have [Poetry](https://python-poetry.org/docs/#installation) installed.
Move to `python/plugins` and execute `poetry new myplugin`.
This will generate a new directory for your plugin.

In the `pyproject.toml` file and add the following line at the end of the `[tool.poetry.dependencies]` section:
```
pydevlake = { path = "../../pydevlake", develop = true }
```

Now run `poetry install`.

## Create `main` file

Create a `main.py` file with the following content:

```python
from typing import Iterable

import pydevlake as dl


class MyPluginConnection(dl.Connection):
    pass


class MyPluginScopeConfig(dl.ScopeConfig):
    pass


class MyPluginToolScope(dl.ToolScope):
    pass


class MyPlugin(dl.Plugin):
    connection_type = MyPluginConnection
    tool_scope_type = MyPluginToolScope
    scope_config_type = MyPluginScopeConfig
    streams = []

    def domain_scopes(self, tool_scope: MyScope) -> Iterable[dl.DomainScope]:
        ...

    def remote_scope_groups(self, connection: MyPluginConnection) -> Iterable[dl.RemoteScopeGroup]:
        ...

    def remote_scopes(self, connection, group_id: str) -> Iterable[MyPluginToolScope]:
        ...

    def test_connection(self, connection: MyPluginConnection) -> dl.TestConnectionResult:
        ...


if __name__ == '__main__':
    MyPlugin.start()
```

This file is the entry point to your plugin.
It specifies three datatypes:
- A connection that groups the parameters that your plugin needs to collect data, e.g. the url and credentials to connect to the datasource
- A tool layer scope type that represents the top-level entity of this plugin, e.g. a board, a repository, a project, etc.
- A scope config that contains the domain entities for a given scope and the the parameters that your plugin uses to convert some data, e.g. regexes to match issue type from name.


The plugin class declares what are its connection, tool scope, and scope config types.
It also declares its list of streams, and is responsible to define 4 methods that we'll cover hereafter.

We also need to create two shell scripts in the plugin root directory to build and run the plugin.
Create a `build.sh` file with the following content:

```bash
#!/bin/bash

cd "$(dirname "$0")"
poetry install
```

And a `run.sh` file with the following content:

```bash
#!/bin/bash

cd "$(dirname "$0")"
poetry run python myplugin/main.py "$@"
```

### Connection parameters

The parameters of your plugin split between those that are required to connect to the datasource that are grouped in your connection class
and those that are used to customize conversion to domain models that are grouped in your scope config class.
For example, to add `url` and `token` parameter, edit `MyPluginConnection` as follow:

```python
from pydantic import SecretStr

class MyPluginConnection(Connection):
    url: str
    token: SecretStr
```

Using type `SecretStr` instead of `str` will encode the value in the database.
To get the `str` value, you need to call `get_secret_value()`: `connection.token.get_secret_value()`.
All plugin methods that have a connection parameter will be called with an instance of this class.
Note that you should not define `__init__`.

### Scope config

A scope config contains the list of domain entities to collect and optionally some parameters used to customize the conversion of data from the tool layer to the domain layer. For example, you can define a regex to match issue type from issue name.

```python
class MyPluginScopeConfig(ScopeConfig):
    issue_type_regex: str
```

If your plugin does not require any such conversion parameter, you can omit this class and the `scope_config_type` plugin attribute.


### Tool scope type

The tool scope type is the top-level entity type of your plugin.
For example, a board, a repository, a project, etc.
A scope is connected to a connection, and all other collected entities are related to a scope.
For example, a plugin for Jira will have a tool scope type of `Board`, and all other entities, such as issues, will belong to a single board.


### Implement domain_scopes method


The `domain_scopes` method should return the list of domain scopes that are related to a given tool scope. Usually, this consists of a single domain scope, but it can be more than one for plugins that collect data from multiple domains.


```python
from pydevlake.domain_layer.devops import CicdScope
...

class MyPlugin(dl.Plugin):
    ...

    def domain_scopes(self, tool_scope: MyPluginToolScope) -> list[dl.DomainScope]:
        yield CicdScope(
            name=tool_scope.name,
            description=tool_scope.description,
            url=tool_scope.url,
        )

```


### Implement `remote_scope` and `remote_scope_groups` method

Those two methods are used by DevLake to list the available scopes in the datasource.
The `remote_scope_groups` method should return a list of scope "groups" and the `remote_scopes` method should return the list of tool scopes in a given group.


```python
class MyPlugin(dl.Plugin):
    ...

    def remote_scope_groups(self, connection: MyPluginConnection) -> Iterable[dl.RemoteScopeGroup]:
        api = ...
        response = ...
        for raw_group in response:
            yield RemoteScopeGroup(
                id=raw_group.id,
                name=raw_group.name,
            )

    def remote_scopes(self, connection, group_id: str) -> Iterable[MyPluginToolScope]:
        api = ...
        response = ...
        for raw_scope in response:
            yield MyPluginToolScope(
                id=raw_scope['id'],
                name=raw_scope['name'],
                description=raw_scope['description'],
                url=raw_scope['url'],
            )
```

### Implement `test_connection` method

The `test_connection` method is used to test if a given connection is valid.
It should check that the connection credentials are valid.
It should make an authenticated request to the API and return a `TestConnectionResult`.
There is a convenience static method `from_api_response` to create a `TestConnectionResult` object from an API response.

```python
class MyPlugin(dl.Plugin):
    ...

    def test_connection(self, connection: MyPluginConnection) -> dl.TestConnectionResult:
        api = ... # Create API client
        response = ... # Make authenticated request to API
        return dl.TestConnection.from_api_response(response)
```


## Add a new data stream

A data stream groups the logic for:
- collecting the raw data from the datasource
- extracting this raw data into a tool-specific model
- converting the tool model into an equivalent [DevLake domain model](https://devlake.apache.org/docs/next/DataModels/DevLakeDomainLayerSchema)


### Create a tool model

Create a `models.py` file.
Then create a class that modelizes the data your stream is going to collect.

```python
from pydevlake.model import ToolModel

class User(ToolModel, table=True):
    id: str = Field(primary_key=True)
    name: str
    email: str
```

Your tool model must declare at least one attribute as a primary key, like `id` in the example above.
It must inherit from `ToolModel`, which in turn inherit from `SQLModel`, the base class of an [ORM of the same name](https://sqlmodel.tiangolo.com/).
You can use `SQLModel` features like [declaring relationships with other models](https://sqlmodel.tiangolo.com/tutorial/relationship-attributes/).
Do not forget `table=True`, otherwise no table will be created in the database. You can omit it for abstract model classes.

To facilitate or even eliminate extraction, your tool models should be close to the raw data you collect. Note that if you collect data from a JSON REST API that uses camelCased properties, you can still define snake_cased attributes in your model. The camelCased attributes aliases will be generated, so no special care is needed during extraction.

#### Migration of tool models

Tool models, connection, scope and scope config types are stored in the DevLake database.
When you create or change the definition of one of those types, the database needs to be migrated. This requires that
you write migration code for them. Important: When you write a migration for a creation operation, the model must be a SNAPSHOT of the models you're intending
to migrate; don't directly use the actual model because that may change over time. Instead, define a model that is a copy of the main one, and use that in the
migration - this model's code will never change (Hence, it's a snapshot).
Also, keep in mind, that Python only supports writing schema migrations. If your flow requires data migrations as well, at this time, the code needs to
be written in Go. See [this Go package](https://github.com/apache/devlake/tree/main/backend/server/services/remote/models/migrationscripts) for example.

To declare a new migration script, you decorate a function with the `migration` decorator. The function name should describe what the script does. The `migration` decorator takes a version number that should be a 14 digits timestamp in the format `YYYYMMDDhhmmss`. The function takes a `MigrationScriptBuilder` as a parameter. This builder exposes methods to execute migration operations.

##### Migration operations

The `MigrationScriptBuilder` exposes several methods. Here we list a few:
- `execute(sql: str, dialect: Optional[Dialect])`: execute a raw SQL statement. The `dialect` parameter is used to execute the SQL statement only if the database is of the given dialect. If `dialect` is `None`, the statement is executed unconditionally.
- `drop_column(table: str, column: str)`: drop a column from a table
- `drop_table(table: str)`: drop a table

Example of creating tables via migrations:
```python

@migration(20230501000001, name="initialize schemas for Plugin")
def init_schemas(b: MigrationScriptBuilder):
    class PluginConnection(Connection):
        token: SecretStr
        organization: Optional[str]
    b.create_tables(PluginConnection)
```


```python
from pydevlake.migration import MigrationScriptBuilder, migration, Dialect

@migration(20230524181430, name="add pk to Job table")
def add_build_id_as_job_primary_key(b: MigrationScriptBuilder):
    table = Job.__tablename__
    b.execute(f'ALTER TABLE {table} DROP PRIMARY KEY', Dialect.MYSQL)
    b.execute(f'ALTER TABLE {table} DROP CONSTRAINT {table}_pkey', Dialect.POSTGRESQL)
    b.execute(f'ALTER TABLE {table} ADD PRIMARY KEY (id, build_id)')
```


### Create the stream class

Create a new file for your first stream in a `streams` directory.

```python
from pydevlake import Stream, DomainType
import pydevlake.domain_layer.crossdomain as cross

from myplugin.models import User


class Users(Stream):
    tool_model = ToolUser
    domain_models = [cross.User]

    def collect(self, state, context) -> Iterable[Tuple[object, dict]]:
        pass

    def extract(self, raw_data) -> ToolUser:
        pass

    def convert(self, user: ToolUser, context) -> Iterable[DomainUser]:
        pass
```

This stream will collect raw user data, e.g. as parsed JSON objects, extract this raw data as your
tool-specific user model, then convert them into domain-layer user models.

The `tool_model` class attribute declares the tool model class that is extracted by this stream.
The `domain_domain` class attribute is a list of domain models that are converted from the tool model.
Most of the time, you will convert a tool model into a single domain model, but need to convert it into multiple domain models.

The `collect` method takes a `state` dictionary and a context object and yields tuples of raw data and new state.
The last state that the plugin yielded for a given connection will be reused during the next collection.
The plugin can use this `state` to store information necessary to perform incremental collection of data. This operates
independently of the way Go manages state, and is tracked by the table `_pydevlake_subtask_runs`. See [this issue](https://github.com/apache/devlake/issues/4880)
for a proposed improvement to this feature.

The `extract` method takes a raw data object and returns a tool model.
This method has a default implementation that populates an instance of the `tool_model` class with the raw data.
When you need to extract a nested value from JSON raw data, you can specify a JSON pointer (see RFC 6901) in the as `source` argument to a `Field` declaration.

```python
class User(ToolModel, table=True):
    id: str = Field(primary_key=True)
    name: str
    email: str
    address: str = Field(source="/contactInfo/address")
```

Here the address field will be populated with the value of the `address` property of the `contactInfo` object property of the JSON object.

The `convert` method takes a tool-specific user model and convert it into domain level user models.
Here the two models align quite well, the conversion is trivial:

```python
def convert(self, user: ToolUser, context: Context) -> Iterable[DomainUser]:
    yield DomainUser(
        id=user.id,
        name=user.name
        email=user.email
    )
```


#### Substreams

Sometimes, a datasource is organized hierarchically. For example, in Jira an issue have many comments.
In this case, you can create a substream to collect the comments of an issue.
A substream is a stream that is executed for each element of a parent stream.
The parent tool model, in our example an issue, is passed to the substream `collect` method as the `parent` argument.

```python
import pydevlake as dl
import pydevlake.domain_layer.ticket as ticket

from myplugin.streams.issues import Issues

class Comments(dl.Substream):
    tool_model = IssueComment
    domain_models = [ticket.IssueComment]
    parent_stream = Issues

    def collect(self, state, context, parent: Issue) -> Iterable[Tuple[object, dict]]:
        ...
```


### Create an API wrapper

Lets assume that your datasource is a REST API.
We can create the following class to define it.

```python
from pydevlake.api import API


class MyAPI(API):
    def __init__(self, url: str):
        self.url = url

    def users(self):
        return self.get(f'{self.url}/users')
```

By inheriting `API` you get access to facilities to wrap REST APIs.
Here the `users` method will return a `Response` object that contains the results of calling `GET` on `<url>/users`.

Now we can go back to our stream file and implement `collect`:

```python
from myplugin.api import MyAPI

...

    def collect(self, state, context) -> Iterable[Tuple[object, dict]]:
        api = MyAPI(context.connection.url)
        for user in api.users().json:
            yield user, state

...
```

If the API responds with a list of JSON object with properties matching your `User` model attributes, you're done!.
Indeed extraction has a default implementation that takes of this common case.
This is why it is important to make tool models that align with the data you collect.

If this is not the case, e.g. the attribute case mismatch, you can redefine the `extract` method:

```python
...

class Users(Stream):
    ...

    def extract(self, raw_data: dict) -> ToolModel:
        return ToolUser(
            id=raw_data["ID"],
            name=raw_data["Name"],
            email=raw_data["Email"]
        )

    ...
```


#### Request and response hook

For each request sent and response received by your API wrapper,
you can register hooks. Hooks allows you to implement
authentication, pagination, and generic API error handling.

For example, lets assume that we are dealing with an API that
require user to authenticate via a token set in a request header.

A request hook is declared in your API with a `@request_hook` decorator.

```python
...
class MyAPI(API):
    def __init__(self, url, token):
        self.url = url
        self.token = token

    ...
    @request_hook
    def authenticate(self, request):
        if self.token:
            request.headers['Token'] = self.token
    ...
```

Here the method `authenticate` is a hook that is run on each request.
Similarly you can declare response hooks with `@response_hook`.
Multiple hooks are executed in the order of their declaration.
The `API` base class declares some hooks that are executed first.


#### Pagination

One usage of a response hook is for handling paginated results.
A response hook can be used to wrap the `Response` object in a
`PagedResponse` object that support iteration and fetching other pages.
This response hook is actually defined in `API` base class and expect
your API wrapper to declare a `paginator` property.

You can subclass `Paginator` to provide API specific logic or reuse an
existing implementation such as `TokenPaginator`.
A token paginator assumes the API paginated responses are JSON object with one
property that is an array of items and another that contains the token to the next
page.

For example, the following paginator fetch items from the `'results'` attribute,
the next page from the `'nextPage'` attribute and will issue requests with a `page`
query parameter.

```
...
class MyAPI(API):
    ...
    paginator = TokenPaginator(
        items_attr='results',
        next_page_token_attr='nextPage',
        next_page_token_param='page'
    )
    ...
```

## Substreams

With REST APIs, you often need to fetch a stream of items, and then to collect additional
data for each of those items.

For example you might want to collect all `UserComments` written by each user collected via the `Users` stream.

To handle such cases, `UserComments` would inherit from `Substream` instead of `Stream`.
A substream needs to specify which is his parent stream. The `collect` method
of a substream will be called with each item collected from the parent stream.

```python
...
from pydevlake import Substream
from myplugin.streams.users import Users

class UserComments(Substream):
    parent_stream = Users # Must specify the parent stream
    ...
    def collect(self, state: dict, context, user: User):
        """
        This method will be called for each user collected from parent stream Users.
        """
        api = MyPluginAPI(context.connection.token.get_secret_value())
        for json in api.user_comments(user.id):
            yield json, state
    ...
```


# Test the plugin standalone

To test your plugin manually, you can run your `main.py` file with different commands.
You can find all those commands with `--help` cli flag:

```console
poetry run myplugin/main.py --help
```

For testing, the interesting commands are `collect`/`extract`/`convert`.
Each takes a context and a stream name.
The context is a JSON object that must at least contain:
- a `db_url`, e.g. you can use `"sqlite+pysqlite:///:memory:"` for an in-memory DB
- a `connection` object containing the same attributes than your plugin connection type

Also, python plugins communicate with go side over an extra file descriptor 3, so you should
redirect to stdout when testing your plugin.

```
console
CTX='{"db_url":"sqlite+pysqlite:///:memory:", "connection": {...your connection attrs here...}}'
poetry run myplugin/main.py $CTX users 3>&1
```


# Automated tests
Make sure you have unit-tests written for your plugin code. The test files should end with `_test.py`, and are discovered and
executed by the `run_tests.sh` script by the CICD automation. The test files should be placed inside the plugin project directory.


# Debugging Python plugins
You need to have a Python remote-debugger installed to debug the Python code. This capability is controlled by the environment
variable `USE_PYTHON_DEBUGGER` which is empty by default. The allowed debuggers as of now are:

- pycharm

You will further have to set the environment variables `PYTHON_DEBUG_HOST` (The hostname/IP on which your debugger is running relative to the environment
in which the plugin is running) and `PYTHON_DEBUG_PORT` (The corresponding debugger port). The variables should be set in the
Go integration tests written in `backend/test/integration/remote` or Docker container/server env configuration. Once done,
set breakpoints in the Python plugin code in your IDE, turn on the debugger in it, and those breakpoints should get hit.