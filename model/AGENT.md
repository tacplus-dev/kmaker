# tacplus Registry

This file is embedded in `registry.json` and describes how to navigate it.

## Overview

This registry describes a tacplus-generated service mesh with:
- **2302** message types across **79** packages
- **1228** RPCs across **131** gRPC services
- **123** state boundaries

## Structure

The registry is a recursive `PackageNode` tree. Each node has:
- `name` — short package name
- `full_path` — slash-separated path from root
- `children` — sub-packages
- `proto_packages` — declared protobuf package names represented by a leaf node
- `vservices` — virtual service identities declared by proto files in a leaf node
- `client_dependencies` — declared `grpc_*_client` edges with `surface_kind`
- `types` — message type descriptors (`TypeInfo`)
- `rpcs` — RPC descriptors (`RpcInfo`)
- `boundaries` — state boundary descriptors (`BoundaryInfo`)

Navigate by traversing `children` to reach any package. Directory paths are navigation
labels; use `proto_packages`, `TypeInfo.package`, and `ServiceInfo.package` for
declared protobuf identity.

## Self-Description

The `$schema` field points to `registry.schema.json` (JSON Schema format).
The type definitions under `sys/tacplus` describe this document's own structure:
`PackageNode`, `TypeInfo`, `FieldInfo`, `RpcInfo`, `BoundaryInfo`, `EnumValueInfo`.

## Generated Field Reference

This field list is generated from `pkg/proto/sys/tacplus.proto`.

### LanguageDependencies
- `dependencies` — map<string, string>


### DependenciesRegistry
- `gitlab_project_id` — string
- `interfaces` — map<string, string>
- `imports` — map<string, string>


### DependenciesConfig
- `registry` — tacplus.DependenciesRegistry
- `tools` — map<string, string>
- `lang` — map<string, tacplus.LanguageDependencies>


### BoundaryInfo
- `service` — tacplus.ServiceName
- `state_root` — string
- `boundary` — string
- `persistence` — bool
- `collection` — bool
- `type_fqn` — string
- `boundary_callers` — repeated string
- `rpcs` — repeated string
- `is_config` — bool
- `label` — repeated string
- `graph_type` — string
- `auto_tls_key_type` — string
- `collected_health` — bool


### SubscriptionInfo
- `source` — string
- `source_package` — string
- `source_state_root` — string
- `source_field` — string
- `source_type_fqn` — string


### EventPublishInfo
- `actor_package` — string
- `actor_message` — string
- `topic_ref` — string
- `event_type_fqn` — string
- `broker_label` — string
- `rpc_stem` — string
- `auth_model` — string
- `auth_services` — repeated string
- `auth_disabled` — bool
- `auth_any_user` — bool
- `target_namespace_field` — string


### EventWatchInfo
- `actor_package` — string
- `actor_message` — string
- `topic_ref` — string
- `event_type_fqn` — string
- `broker_label` — string
- `rpc_stem` — string
- `auth_model` — string
- `auth_services` — repeated string
- `auth_disabled` — bool
- `auth_any_user` — bool
- `target_namespace_field` — string


### EnumValueInfo
- `name` — string
- `number` — int32
- `description` — string


### FieldConstraintInfo
- `rule` — string
- `target` — string
- `value_kind` — string
- `bool_value` — bool
- `int_value` — int64
- `float_value` — double
- `string_value` — string
- `enum_value` — string
- `int_values` — repeated int64
- `float_values` — repeated double
- `operator` — string
- `defaulted` — bool
- `unit` — string
- `source` — string


### FieldInfo
- `name` — string
- `type` — spec.TypeKind
- `description` — string
- `required` — bool
- `repeated` — bool
- `default_json` — string
- `constraints` — repeated string
- `message` — tacplus.TypeInfo
- `enum_values` — repeated tacplus.EnumValueInfo
- `map_key_type` — string
- `map_value_type` — string
- `projection_source` — string
- `timeseries_order_key` — int32
- `timeseries_partition_key` — bool
- `oneof` — bool
- `oneof_name` — string
- `store_key` — bool
- `quorum_write` — bool
- `constraint_infos` — repeated tacplus.FieldConstraintInfo
- `timeseries_cardinality_hint` — spec.CardinalityHint
- `data_entitlement_mode` — spec.RedactMode
- `filter_mode` — spec.QueryOp
- `store_principal` — bool
- `store_namespace` — bool
- `record_key` — bool


### TypeInfo
- `fqn` — string
- `name` — string
- `package` — string
- `description` — string
- `schema_json` — string
- `fields` — repeated tacplus.FieldInfo
- `timeseries_table_name` — string
- `timeseries_table_source` — string
- `record_key` — string
- `record_owner` — string
- `transformations` — repeated tacplus.TransformationInfo
- `service_store` — bool
- `service_store_name` — string
- `service_store_label` — repeated string
- `timeseries_dedup` — tacplus.DedupPolicyInfo
- `data_entitlement_default` — spec.RedactMode


### TransformationFieldInfo
- `source` — string
- `as` — string
- `order_key` — int32
- `partition_key` — bool
- `cardinality_hint` — spec.CardinalityHint


### DedupPolicyInfo
- `strategy` — spec.DedupStrategy
- `key_fields` — repeated string
- `version_field` — string


### TransformationInfo
- `name` — string
- `when` — string
- `field` — repeated tacplus.TransformationFieldInfo
- `dedup` — tacplus.DedupPolicyInfo


### RpcSurfaceInfo
- `surface_kind` — string
- `transport` — string
- `auth_model` — string
- `auth_disabled` — bool
- `auth_services` — repeated string
- `auth_entitled_users` — bool
- `auth_any_user` — bool
- `target_namespace_field` — string
- `subject_namespace_source` — string
- `target_namespace_source` — string
- `target_namespace` — string
- `grpc_path` — string
- `http_path` — string
- `http_methods` — repeated string
- `http_alias_path` — string
- `tenant_scope` — string


### RpcInfo
- `service` — tacplus.ServiceName
- `service_name` — string
- `rpc` — string
- `input_type` — string
- `output_type` — string
- `client_streaming` — bool
- `server_streaming` — bool
- `description` — string
- `category` — tacplus.RpcCategory
- `grpc_path` — string
- `http_path` — string
- `http_methods` — repeated string
- `http_alias_path` — string
- `auth_disabled` — bool
- `auth_services` — repeated string
- `auth_actors` — repeated string
- `auth_entitled_users` — bool
- `auth_any_user` — bool
- `auth_model` — string
- `collection_mode` — string
- `collection_max_items` — uint32
- `collection_order_by` — repeated string
- `collection_replay_depth` — uint32
- `collection_default_page_size` — uint32
- `rpc_verb` — string
- `rpc_noun` — string
- `method_pattern` — string
- `access_kind` — string
- `verb_class` — string
- `entitlement_scopes` — repeated string
- `retry_policy` — string
- `capability_scope` — string
- `verb_definition` — string
- `verb_boundary` — string
- `audit_meaning` — string
- `rejected_verbs` — repeated string
- `allowed_patterns` — repeated string
- `handler_shape` — string
- `target_namespace_field` — string
- `subject_namespace_source` — string
- `target_namespace_source` — string
- `target_namespace` — string
- `verb_status` — string
- `folded_to_verb` — string
- `surfaces` — repeated tacplus.RpcSurfaceInfo
- `mcp` — bool
- `mcp_tool_name` — string
- `mcp_session_carrier` — bool
- `tenant_scope` — string


### EnumInfo
- `name` — string
- `package` — string
- `description` — string
- `values` — repeated tacplus.EnumValueInfo


### ServiceInfo
- `name` — string
- `description` — string
- `package` — string
- `grpc_service` — string
- `rpcs` — repeated tacplus.RpcInfo
- `boundaries` — repeated string
- `event_label` — string
- `smb` — bool
- `smb_authority` — bool
- `smb_root` — spec.SmbRoot
- `host_label` — string
- `mcp` — bool
- `mcp_exposure` — spec.RpcExposure
- `tenant_owned` — bool
- `pinned_tenant` — string
- `volume_groups` — repeated tacplus.VolumeGroupInfo


### VolumeGroupInfo
- `name` — string
- `max_blob_bytes` — uint64
- `content_types` — repeated string
- `tenant_quota_bytes` — uint64


### VServiceInfo
- `name` — string
- `id` — int32
- `package` — string


### ClientDependencyInfo
- `source_package` — string
- `surface_kind` — string
- `service` — string
- `target_package` — string
- `target_service` — string
- `wildcard` — bool


### EntitlementGrantInfo
- `token` — string
- `package` — string
- `service_name` — string
- `rpc` — string
- `capability_scope` — string
- `expands_to` — repeated string
- `routing_alias_packages` — repeated string


### BuildInfo
- `tacplus_version` — string
- `build_timestamp` — string
- `source_git_sha` — string


### VerificationHarness
- `message_fqn` — string
- `harness_name` — string
- `assumptions` — repeated string
- `contracts` — repeated string
- `status` — string
- `detail` — string


### VerificationReport
- `tool` — string
- `tool_version` — string
- `language` — string
- `harnesses` — repeated tacplus.VerificationHarness
- `passed` — int32
- `failed` — int32
- `total` — int32
- `duration_ms` — int64


### PackageNode
- `name` — string
- `full_path` — string
- `children` — repeated tacplus.PackageNode
- `types` — repeated tacplus.TypeInfo
- `rpcs` — repeated tacplus.RpcInfo
- `boundaries` — repeated tacplus.BoundaryInfo
- `docs` — map<string, string>
- `schema` — string
- `enums` — repeated tacplus.EnumInfo
- `services` — repeated tacplus.ServiceInfo
- `build` — tacplus.BuildInfo
- `subscriptions` — repeated tacplus.SubscriptionInfo
- `constants` — map<string, string>
- `entitlement_grants` — repeated tacplus.EntitlementGrantInfo
- `proto_packages` — repeated string
- `vservices` — repeated tacplus.VServiceInfo
- `client_dependencies` — repeated tacplus.ClientDependencyInfo
- `event_publishes` — repeated tacplus.EventPublishInfo
- `event_watches` — repeated tacplus.EventWatchInfo



## Field Metadata

`FieldInfo` includes protobuf oneof grouping:
- `oneof` — true when the field is a member of a protobuf oneof group
- `oneof_name` — the declared oneof group name

`FieldInfo.constraint_infos` contains machine-readable validation constraints
derived from proto annotations and compiler default bounds. Prefer
`constraint_infos` over parsing the legacy prose strings in `constraints`.

Durable `service_store` messages are surfaced in metadata:
- `TypeInfo.service_store` / `service_store_name` / `service_store_label`
- `FieldInfo.store_key` — the key field for a service_store value
- `FieldInfo.quorum_write` — field writes require quorum acknowledgement

## VService Metadata

`PackageNode.vservices` lists file-level `(spec.vservice)` identities with
`name`, `id`, and declaring `package`. Hosted vservice RPC surfaces expose their
hosting label on `ServiceInfo.host_label`.

## Client Dependencies

`PackageNode.client_dependencies` lists file-level `grpc_s2s_client`,
`grpc_c2s_client`, `grpc_w2s_client`, and `all_grpc_s2s_clients` declarations.
Each edge includes `source_package`, `surface_kind`, `service`,
`target_package`, `target_service`, and `wildcard`.

## Event Topology

`PackageNode.event_publishes` and `PackageNode.event_watches` list validated
actor bindings from `(spec.event_publish)` and `(spec.event_watch)`. Each entry
includes the actor package/message, canonical `topic_ref`, resolved
`event_type_fqn`, broker label, RPC stem, and derived auth shape.

## RPC Surfaces

`RpcInfo.auth_model` is the primary annotated service model (`s2s`, `c2s`, or
`c2d`). `RpcInfo.surfaces` lists every generated transport/auth surface for an
RPC. Use it when you need additive exposure such as W2S gRPC-over-REST:
`surface_kind` / `auth_model` may be `w2s`, `transport` is `grpc_over_rest`,
and the row carries the W2S HTTP route plus namespace/auth metadata.

## RPC Classification

Every RPC may carry classifier metadata derived from the governed verb registry:
- `auth_model` — primary annotated service model: `s2s`, `c2s`, or `c2d`
- `verb_status` — `classified`, `unclassified`, or `exempt`
- `rpc_verb` / `rpc_noun` — the canonical `Verb<Noun>` split when classification succeeds
- `method_pattern` — the static shape (`GET`, `LIST`, `WATCH`, `CREATE`, `UPDATE`,
  `DELETE`, `ACTION`, `CLIENT_STREAM`, or `BIDI`)
- `access_kind` — `READ` or `WRITE`
- `verb_class` — `core`, `action`, or `agent`
- `entitlement_scopes` — grant scopes that admit the method, ordered broad to narrow
- `retry_policy` — semantic retry/idempotency posture
- `capability_scope` — primary package capability bucket for capability-any expansion
- `verb_definition`, `verb_boundary`, `audit_meaning` — governed dictionary metadata
- `rejected_verbs` — non-canonical verbs that fold to the canonical verb and are rejected
- `folded_to_verb` — canonical verb for a rejected leading synonym, when known
- `allowed_patterns` — patterns this canonical verb may resolve to
- `handler_shape` — `UNARY`, `SERVER_STREAM`, `CLIENT_STREAM`, or `BIDI`

The legacy `category` field is also present for older policy code:
- **ACCESSOR** — read/observe operations (Get, Watch, Query, KeySet, Size, ContainsKey, List, Peek, Subscribe)
- **MUTATOR** — write/modify operations (Set, Put, Delete, Remove, Clear, Publish, Offer, Reload)
- **EXECUTOR** — action/command operations (Assist, Authenticate, Authorize, Evaluate, Exchange, etc.)

Per-service aggregate enum values (`ALL_ACCESSORS`, `ALL_MUTATORS`, `ALL_EXECUTORS`) are available for security policy rules.

## Security Metadata

Each `BoundaryInfo` carries:
- `boundary_callers` — which services can call this boundary cross-namespace

Each `RpcInfo` carries:
- `auth_disabled` — whether authentication is bypassed
- `auth_services` — S2S caller service allowlist when applicable
- `auth_entitled_users` — C2S entitlement-checked user access
- `auth_any_user` — C2S authenticated-user access without entitlement checks
- `auth_model` — primary annotated service model (`s2s`, `c2s`, or `c2d`)
- `target_namespace_field`, `subject_namespace_source`,
  `target_namespace_source`, `target_namespace` — namespace authorization inputs
- `tenant_scope` — S2S record query plane only: `fleet` (every authorized caller
  reads across all tenants) or `caller_pinned` (a `/system` caller stays
  fleet-wide, a `/tenants/<t>` caller reads only `<t>`, decided at runtime from
  the caller's own token claim). Empty on every other rpc
- `surfaces` — per-transport auth metadata, including derived W2S rows
- `category` — legacy accessor/mutator/executor classification
- `rpc_verb`, `rpc_noun`, `method_pattern`, `access_kind`, `verb_class`,
  `entitlement_scopes`, `retry_policy`, `capability_scope`, `verb_definition`,
  `verb_boundary`, `audit_meaning`, `rejected_verbs`, `allowed_patterns`,
  `handler_shape` — governed verb/pattern metadata when classification succeeds

## Services

- **AccountingRecordBusStateService**
- **Admin**
- **AgentCursorBusStateService**
- **AgentCycleBusStateService**
- **AgentEventBusStateService**
- **AgentSessionStateBusStateService**
- **AlertBusStateService**
- **Auth**
- **BeagleConfigStateService**
- **BlobBusStateService**
- **BlobLinkBusStateService**
- **BlobLinkStatsBusStateService**
- **BlobStore**
- **BlobstoreConfigStateService**
- **CaStateService**
- **ChatBusStateService**
- **ChatEventBusStateService**
- **ChatMembershipBusStateService**
- **ChatSessionBusStateService**
- **CliTokenBroker**
- **ClusterService**
- **ClusterStateService**
- **CompanyStructureBusStateService**
- **ConfigStateService**
- **CrewCursorBusStateService**
- **CrewEventBusStateService**
- **CrewRoomMessageBusStateService**
- **Dashboard**
- **Device**
- **DocFileMetadataBusStateService**
- **DocHashIndexBusStateService**
- **DocManager**
- **DocmanagerConfigStateService**
- **EquipmentBusStateService**
- **EventbusConfigStateService**
- **EventbusStateService**
- **FencedLock**
- **ForecastBusStateService**
- **GatewayConfigStateService**
- **GrantBusStateService**
- **HumanBusStateService**
- **IdentityStateService**
- **IngestPipelineBusStateService**
- **JobBidBusStateService**
- **JobBusStateService**
- **JobEquipmentBusStateService**
- **JobPermitsBusStateService**
- **JwkVerifyingKeysViewService**
- **Kmaker**
- **KmakerConfigStateService**
- **LetsEncryptCertBusStateService**
- **LetsEncryptStateService**
- **LetsencryptConfigStateService**
- **LlmProviderSecretBusStateService**
- **LlmProviderSettingsBusStateService**
- **LlmProxy**
- **LlmSettings**
- **LlmproxyConfigStateService**
- **LlmsettingsConfigStateService**
- **Locate811TicketBusStateService**
- **LogbookEntryBusStateService**
- **Mailbox**
- **MailboxConfigStateService**
- **MailboxEntryBusStateService**
- **MessageBusStateService**
- **Messaging**
- **MessagingConfigStateService**
- **Mint**
- **OauthConfigStateService**
- **ObservationBusStateService**
- **OpaConfigStateService**
- **OpaEval**
- **OpaStateService**
- **OperatorConfigStateService**
- **OttoConfigStateService**
- **PartnerBusStateService**
- **PermitBusStateService**
- **PersonalIdentity**
- **PersonalauthConfigStateService**
- **PlaceBusStateService**
- **PolicyVersionBusStateService**
- **PrincipalStateService**
- **Profile**
- **ProfileConfigStateService**
- **PublicJwksViewService**
- **RecordMap**
- **ReportBusStateService**
- **Reveille**
- **ReveilleConfigStateService**
- **RfpBusStateService**
- **RuleNodeBusStateService**
- **RunCheckpointBusStateService**
- **Scotty**
- **ScottyActivityBusStateService**
- **ScottyAgent**
- **ScottyConfigStateService**
- **ScottyPersonal**
- **ScottyRecords**
- **ScottyagentConfigStateService**
- **ScottypersonalConfigStateService**
- **ScottypersonalRecords**
- **SentinelConfigStateService**
- **ServiceConfigStateService**
- **ServiceHealthStateService**
- **Session**
- **SmbCertService**
- **StratumConfigStateService**
- **TallyConfigStateService**
- **TaskBusStateService**
- **TenantAdmin**
- **ThreadMapEntryBusStateService**
- **TicketAdviceBusStateService**
- **TicketBusStateService**
- **TicketCommentBusStateService**
- **TicketDecisionBusStateService**
- **TlsCertService**
- **ToolCallOutcomeBusStateService**
- **TraceEventBusStateService**
- **TraceSession**
- **TrustedCaStateService**
- **UploadSessionBusStateService**
- **UserMint**
- **UserProfileBusStateService**
- **WaitBusStateService**
- **Weather**
- **WeatherConfigStateService**
- **Work**
- **WorkActivityBusStateService**
- **WorkConfigStateService**
- **WorkRecords**
- **WorkloadStateService**

## Packages

- `clients/harness`
- `clients/schematicctl`
- `common/credentials`
- `common/hazelcast`
- `generated/accounting_method`
- `generated/entitlements`
- `generated/eventbus_support`
- `generated/project_config`
- `generated/services`
- `generated/services_config`
- `generated/tenant_config`
- `services/beagle`
- `services/blobstore`
- `services/docmanager`
- `services/eventbus`
- `services/gateway`
- `services/kernel`
- `services/kmaker`
- `services/letsencrypt`
- `services/llmproxy`
- `services/llmsettings`
- `services/mailbox`
- `services/messaging`
- `services/oauth`
- `services/opa`
- `services/operator`
- `services/otto`
- `services/personalauth`
- `services/profile`
- `services/reveille`
- `services/scotty`
- `services/scottyagent`
- `services/scottypersonal`
- `services/sentinel`
- `services/stratum`
- `services/tally`
- `services/weather`
- `services/work`
- `shared/crew`
- `shared/pigeon`
- `sys/accounting`
- `sys/alpn`
- `sys/cloudidentity`
- `sys/cloudvendor`
- `sys/credpair`
- `sys/diskstore`
- `sys/entitlement`
- `sys/errors`
- `sys/filesystem`
- `sys/grpcconfig`
- `sys/healthreport`
- `sys/host`
- `sys/http`
- `sys/httpclient`
- `sys/ip`
- `sys/jwk`
- `sys/jwkview`
- `sys/key`
- `sys/logger`
- `sys/mcp`
- `sys/opalib`
- `sys/outputformat`
- `sys/publicjwksview`
- `sys/query`
- `sys/smbcert`
- `sys/spec`
- `sys/tacplus`
- `sys/tags`
- `sys/tlscert`
- `sys/token`
- `sys/tokencache`
- `sys/tokenrequest`
- `sys/trace`
- `sys/x509`
- `vservices/accounting-record`
- `vservices/agent-sensor`
- `vservices/entitlements`
- `vservices/proxy-policy`
- `vservices/query`

## Documentation

Additional documentation is available in the `docs` map at the root of this registry.
