# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# NOTE: you have to use tabs in this file for make. Not spaces.
# https://stackoverflow.com/questions/920413/make-error-missing-separator
# https://tutorialedge.net/golang/makefiles-for-go-developers/

SHA ?= $(shell if [ -d .git ]; then git show -s --format=%h; else echo "unknown_sha"; fi)
TAG ?= $(shell if [ -d .git ]; then git tag --points-at HEAD; else echo "local_build"; fi)
IMAGE_REPO ?= "apache"
VERSION = $(TAG)@$(SHA)

build-server-image:
	make build-server-image -C backend

build-config-ui-image:
	cd config-ui; docker build -t $(IMAGE_REPO)/devlake-config-ui:$(TAG) --file ./Containerfile .

build-grafana-image:
	cd grafana; docker build -t $(IMAGE_REPO)/devlake-dashboard:$(TAG) --file ./Dockerfile .

build-images: build-server-image build-config-ui-image build-grafana-image

push-server-image: build-server-image
	docker push $(IMAGE_REPO)/devlake:$(TAG)

push-config-ui-image: build-config-ui-image
	docker push $(IMAGE_REPO)/devlake-config-ui:$(TAG)

push-grafana-image: build-grafana-image
        docker push $(IMAGE_REPO)/devlake-dashboard:$(TAG)

push-images: push-server-image push-config-ui-image push-grafana-image

configure:
	docker-compose up config-ui

configure-dev:
	cd config-ui; yarn; yarn start

commit:
	git cz

restart:
	docker-compose down; docker-compose up -d

# Actually execute in ./backend
go-dep:
	make go-dep -C backend

python-dep:
	make python-dep -C backend

dep: go-dep python-dep

swag:
	make swag -C backend

build-plugin:
	make build-plugin -C backend

build-server:
	make build-server -C backend

build: build-plugin build-server

all: build

tap-models:
	make tap-models -C backend

run:
	make run -C backend

dev:
	make dev -C backend

godev:
	make godev -C backend

debug:
	make debug -C backend

mock:
	make mock -C backend

test: unit-test e2e-test

unit-test: mock unit-test-only

unit-test-only:
	make unit-test -C backend

python-unit-test:
	make python-unit-test -C backend

e2e-test:
	make e2e-test -C backend

e2e-plugins-test:
	make e2e-plugins-test -C backend

integration-test:
	make integration-test -C backend

lint:
	make lint -C backend

fmt:
	make fmt -C backend

clean:
	make clean -C backend
