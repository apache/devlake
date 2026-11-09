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


from typing import Type, get_args, get_origin, Union
from types import NoneType

from pydantic_core import PydanticUndefined
from jsonpointer import resolve_pointer, JsonPointerException

from pydevlake import ToolModel


def _is_optional(field_info) -> bool:
    """Check if a field's annotation allows None (is Optional)."""
    annotation = field_info.annotation
    if annotation is None:
        return True
    origin = get_origin(annotation)
    if origin is Union:
        return NoneType in get_args(annotation)
    return False


def autoextract(json: dict, model_cls: Type[ToolModel]) -> ToolModel:
    """
    Automatically extract a tool model from a json object.
    The tool model class can define fields with a source argument to specify the JSON pointer (RFC 6901) to the value.

    Example:
        class DummyModel(ToolModel):
            name: str
            version: str = Field(source='/version/number')

        json = {
            'name': 'test',
            'version': {
                'number': '1.0.0',
                'build_date': '2023-04-19'
            }
        }

        model = autoextract(json, DummyModel)
    """
    attributes = {}
    for field_name, field_info in model_cls.model_fields.items():
        # In SQLModel 0.0.38, schema_extra is stored in field_info.json_schema_extra
        extra = field_info.json_schema_extra or {}
        if isinstance(extra, dict):
            pointer = extra.get('source')
        else:
            pointer = None

        if pointer:
            # A field is considered optional if its annotation allows None
            is_optional = _is_optional(field_info)
            if not is_optional:
                try:
                    value = resolve_pointer(json, pointer)
                except JsonPointerException:
                    raise ValueError(f"Missing required value for field {field_name} at {pointer}")
            else:
                default = field_info.default if field_info.default is not PydanticUndefined else None
                try:
                    value = resolve_pointer(json, pointer, default)
                except JsonPointerException:
                    value = default
        else:
            alias = field_info.alias
            value = json.get(field_name) or json.get(alias) if alias else json.get(field_name)
        attributes[field_name] = value
    # Use model_validate so values are coerced (e.g. str -> datetime/Enum).
    # SQLModel table=True models skip validation on __init__, which would leave
    # raw strings uncoerced under Pydantic v2, so validate explicitly here.
    return model_cls.model_validate(attributes)
