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

from pydevlake.migration import migration, MIGRATION_SCRIPTS


@migration(20230520174322)
def my_migration(b):
    b.execute("SOME SQL")
    b.drop_column("t", "c")
    b.drop_table("t")


def test_migration():
    assert my_migration.version == 20230520174322
    assert my_migration.name == "my_migration"
    assert len(my_migration.operations) == 3

    op1 = my_migration.operations[0]
    assert op1.sql == "SOME SQL"
    assert op1.dialect is None

    op2 = my_migration.operations[1]
    assert op2.table == "t"
    assert op2.column == "c"

    op3 = my_migration.operations[2]
    assert op3.table == "t"


def test_registration():
    assert my_migration in MIGRATION_SCRIPTS


def test_serialization():
    val = my_migration.model_dump()
    assert val["version"] == 20230520174322
    assert val["name"] == "my_migration"
    assert len(val["operations"]) == 3

    op1 = val["operations"][0]
    assert op1["type"] == "execute"
    assert op1["sql"] == "SOME SQL"
    assert "dialect" not in op1 or op1["dialect"] is None

    op2 = val["operations"][1]
    assert op2["type"] == "drop_column"
    assert op2["table"] == "t"
    assert op2["column"] == "c"

    op3 = val["operations"][2]
    assert op3["type"] == "drop_table"
    assert op3["table"] == "t"
