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
#Apache DevLake is an effort undergoing incubation at The Apache Software
#Foundation (ASF), sponsored by the Apache Incubator PMC.
#
#Incubation is required of all newly accepted projects until a further review
#indicates that the infrastructure, communications, and decision making process
#have stabilized in a manner consistent with other successful ASF projects.
#
#While incubation status is not necessarily a reflection of the completeness or stability of the code,
#it does indicate that the project has yet to be fully endorsed by the ASF.

FROM grafana/grafana:11.6.2
COPY ./provisioning/dashboards /etc/grafana/provisioning/dashboards
COPY ./dashboards /etc/grafana/dashboards
COPY ./scripts/entrypoint.sh /entrypoint.sh
COPY ./img/grafana_icon.svg /usr/share/grafana/public/img/grafana_icon.svg
COPY ./img /usr/share/grafana/public/img/lake
ENV GF_USERS_ALLOW_SIGN_UP=false
ENV GF_SERVER_SERVE_FROM_SUB_PATH=true
ENV GF_DASHBOARDS_JSON_ENABLED=true
ENV GF_LIVE_ALLOWED_ORIGINS='*'
USER root
RUN grafana-cli plugins install grafana-piechart-panel
RUN chmod +x /entrypoint.sh && \
    chgrp -R 0 /etc/grafana /usr/share/grafana /var/lib/grafana && \
    chmod -R g=u /etc/grafana /usr/share/grafana /var/lib/grafana
USER 101
ENTRYPOINT ["/entrypoint.sh"]
