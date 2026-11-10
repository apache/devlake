# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at

#     http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


import json

import pytest
from sqlmodel import SQLModel, Session, Field, create_engine

from pydevlake import Stream, Connection, Context, DomainType
from pydevlake.model import ScopeConfig, ToolModel, DomainModel, ToolScope, raw_data_params


class DummyToolModel(ToolModel, table=True):
    id: int = Field(primary_key=True)
    name: str


class DummyDomainModel(DomainModel, table=True):
    Name: str


class DummyStream(Stream):
    tool_model = DummyToolModel
    domain_types = [DomainType.CROSS]

    def collect(self, state, context):
        for i, each in enumerate(context.connection.raw_data):
            count = state.get("count", 0)
            yield each, {"count": count + i}

    def extract(self, raw) -> ToolModel:
        return DummyToolModel(
            id=raw["i"],
            name=raw["n"]
        )

    def convert(self, tm, ctx):
        return DummyDomainModel(
            ID=tm.id,
            Name=tm.name,
        )


class DummyConnection(Connection):
    raw_data: list[dict]


@pytest.fixture
def engine():
    engine = create_engine("sqlite+pysqlite:///:memory:")
    SQLModel.metadata.create_all(engine)
    return engine

@pytest.fixture
def raw_data():
    return [
        {"i": 1, "n": "alice"},
        {"i": 2, "n": "bob"}
    ]


@pytest.fixture
def connection(raw_data):
    return DummyConnection(id=11, name='dummy connection', raw_data=raw_data)


@pytest.fixture
def scope():
    return ToolScope(id='scope_id', name='scope_name')


@pytest.fixture
def ctx(connection, scope, engine):
    return Context(
        engine=engine,
        scope=scope,
        connection=connection,
        scope_config=ScopeConfig(),
        options={}
    )


@pytest.fixture
def stream():
    return DummyStream("test")


def test_collect_data(stream, raw_data, ctx):
    gen = stream.collector.run(ctx)
    list(gen)

    with Session(ctx.engine) as session:
        raw_model = stream.raw_model(session)
        all_raw = [json.loads(r.data) for r in session.query(raw_model).all()]
        assert all_raw == raw_data


def test_extract_data(stream, raw_data, ctx):
    with Session(ctx.engine) as session:
        for each in raw_data:
            raw_model = stream.raw_model(session)
            raw_model.params = raw_data_params(ctx.connection.id, ctx.scope.id)
            session.add(raw_model(data=json.dumps(each).encode('utf8')))
        session.commit()

    gen = stream.extractor.run(ctx)
    list(gen)

    tool_models = session.query(DummyToolModel).all()
    alice = tool_models[0]
    bob = tool_models[1]
    assert alice.name == 'alice'
    assert alice.id == 1

    assert bob.name == 'bob'
    assert bob.id == 2


def test_convert_data(stream, raw_data, ctx):
    with Session(ctx.engine) as session:
        for each in raw_data:
            session.add(
                DummyToolModel(
                    id=each["i"],
                    connection_id=ctx.connection.id,
                    name=each["n"],
                    raw_data_table="_raw_dummy_model",
                    raw_data_params=raw_data_params(ctx.connection.id, ctx.scope.id)
                )
            )
        session.commit()

    gen = stream.convertor.run(ctx)
    list(gen)

    tool_models = session.query(DummyDomainModel).all()
    alice = tool_models[0]
    bob = tool_models[1]
    assert alice.Name == 'alice'
    assert alice.id == 'tests:DummyToolModel:11:1'

    assert bob.Name == 'bob'
    assert bob.id == 'tests:DummyToolModel:11:2'
