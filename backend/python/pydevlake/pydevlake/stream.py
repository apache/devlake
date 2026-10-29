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


from typing import Iterable, Type, Optional
from abc import abstractmethod

from pydevlake.context import Context
from pydevlake.subtasks import Collector, Extractor, Convertor, SubstreamCollector
from pydevlake.model import RawModel, ToolModel, ToolScope, DomainModel, DomainType
from pydevlake.extractor import autoextract

_raw_model_classes: dict[str, type] = {}


class Stream:
    def __init__(self, plugin_name: str):
        self.plugin_name = plugin_name
        self.collector = Collector(self)
        self.extractor = Extractor(self)
        self.convertor = Convertor(self)
        self._raw_model = None

    @property
    def subtasks(self):
        return [self.collector, self.extractor, self.convertor]

    @property
    def name(self):
        return type(self).__name__.lower()

    @property
    def qualified_name(self):
        return f'{self.plugin_name}_{self.name}'

    @property
    def tool_model(self) -> Type[ToolModel]:
        pass

    @property
    @abstractmethod
    def domain_types(self) -> list[DomainType]:
        pass

    @property
    @abstractmethod
    def domain_models(self) -> Optional[list[Type[DomainModel]]]:
        pass

    def raw_model(self, session) -> Type[RawModel]:
        model = self._get_raw_model_class()
        table = RawModel.metadata.tables[self.raw_model_table]
        table.create(session.get_bind(), checkfirst=True)
        return model

    def _get_raw_model_class(self) -> Type[RawModel]:
        if self._raw_model is not None:
            return self._raw_model

        table_name = self.raw_model_table
        cached = _raw_model_classes.get(table_name)
        if cached is not None:
            self._raw_model = cached
            return cached

        for mapper in RawModel._sa_registry.mappers:
            model = mapper.class_
            if getattr(model, '__tablename__', None) == table_name:
                _raw_model_classes[table_name] = model
                self._raw_model = model
                return model

        table_args = {'extend_existing': True} if table_name in RawModel.metadata.tables else None

        class StreamRawModel(RawModel, table=True):
            __tablename__ = table_name
            if table_args is not None:
                __table_args__ = table_args

        _raw_model_classes[table_name] = StreamRawModel
        self._raw_model = StreamRawModel
        return StreamRawModel

    @property
    def raw_model_table(self):
        return f'_raw_{self.plugin_name}_{self.name}'

    def collect(self, state, context) -> Iterable[tuple[object, dict]]:
        pass

    def extract(self, raw_data: dict) -> ToolModel:
        return autoextract(raw_data, self.tool_model)

    def convert(self, tool_model: ToolModel, context: Context) -> DomainModel:
        pass

    def should_run_on(self, scope: ToolScope) -> bool:
        return True


class Substream(Stream):
    def __init__(self, plugin_name: str):
        super().__init__(plugin_name)
        self.collector = SubstreamCollector(self)

    @property
    @abstractmethod
    def parent_stream(self):
        pass

    def collect(self, state, context, parent) -> Iterable[tuple[object, dict]]:
        pass
