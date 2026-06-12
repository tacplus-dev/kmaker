# tacplus Registry

This file is embedded in `registry.json` and describes how to navigate it.

## Overview

This registry describes a tacplus-generated service mesh with:
- **1244** message types across **71** packages
- **738** RPCs across **71** gRPC services
- **67** state boundaries

## Structure

The registry is a recursive `PackageNode` tree. Each node has:
- `name` — short package name
- `full_path` — slash-separated path from root
- `children` — sub-packages
- `types` — message type descriptors (`TypeInfo`)
- `rpcs` — RPC descriptors (`RpcInfo`)
- `boundaries` — state boundary descriptors (`BoundaryInfo`)

Navigate by traversing `children` to reach any package. The tree mirrors the proto package hierarchy.

## Self-Description

The `$schema` field points to `registry.schema.json` (JSON Schema format).
The type definitions under `libs/tacplus` describe this document's own structure:
`PackageNode`, `TypeInfo`, `FieldInfo`, `RpcInfo`, `BoundaryInfo`, `EnumValueInfo`, `StatePermEntry`.

## RPC Classification

Every RPC may carry classifier metadata derived from the governed verb registry:
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
- `allowed_patterns` — patterns this canonical verb may resolve to
- `handler_shape` — `UNARY`, `SERVER_STREAM`, `CLIENT_STREAM`, or `BIDI`

The legacy `category` field is also present for older policy code:
- **ACCESSOR** — read/observe operations (Get, Watch, Query, KeySet, Size, ContainsKey, List, Peek, Subscribe)
- **MUTATOR** — write/modify operations (Set, Put, Delete, Remove, Clear, Publish, Offer, Reload)
- **EXECUTOR** — action/command operations (Assist, Authenticate, Authorize, Evaluate, Exchange, etc.)

Per-service aggregate enum values (`ALL_ACCESSORS`, `ALL_MUTATORS`, `ALL_EXECUTORS`) are available for security policy rules.

## Security Metadata

Each `BoundaryInfo` carries:
- `state_perms` — which other state roots this service accesses (target + mode: RO/RW/WORM/CALL)
- `internal` — namespace isolation flag
- `boundary_callers` — which services can call this boundary cross-namespace
- `boundary_perms` — per-boundary access overrides

Each `RpcInfo` carries:
- `disable_auth` — whether authentication is bypassed
- `allowed_callers` — service-level caller restrictions
- `allow_user` — whether end-user tokens are accepted
- `category` — legacy accessor/mutator/executor classification
- `rpc_verb`, `rpc_noun`, `method_pattern`, `access_kind`, `verb_class`,
  `entitlement_scopes`, `retry_policy`, `capability_scope`, `verb_definition`,
  `verb_boundary`, `audit_meaning`, `rejected_verbs`, `allowed_patterns`,
  `handler_shape` — governed verb/pattern metadata when classification succeeds

## Services

- **Admin**
- **AgenticCodeEventBusStateService**
- **AnthropicConfigStateService**
- **Argus**
- **ArgusConfigStateService**
- **Assist**
- **Auth**
- **CaStateService**
- **CliTokenBroker**
- **ClusterService**
- **ClusterStateService**
- **ConfigStateService**
- **ConnectorStatus**
- **DaemonControl**
- **Dashboard**
- **EventbusConfigStateService**
- **EventbusStateService**
- **GatewayConfigStateService**
- **GatewayJwksViewService**
- **GatewayStateService**
- **GitLabConnectorEventBusStateService**
- **GitLabConnectorStateService**
- **GithubConfigStateService**
- **GitlabConfigStateService**
- **GitlabconnectorConfigStateService**
- **GraphMutation**
- **GraphdbConfigStateService**
- **GraphdbQuery**
- **GraphdbStateService**
- **HookReceiver**
- **IdentityStateService**
- **Inbound**
- **JwkVerifyingKeysViewService**
- **Kmaker**
- **KmakerConfigStateService**
- **LetsEncryptStateService**
- **LetsencryptConfigStateService**
- **LlmEventBusStateService**
- **LockService**
- **McpConfigStateService**
- **Messaging**
- **Metadata**
- **Mint**
- **MsggatewayConfigStateService**
- **OauthConfigStateService**
- **OpaConfigStateService**
- **OpaEval**
- **OpaStateService**
- **OpenaiConfigStateService**
- **OperatorConfigStateService**
- **Outbound**
- **Proxy**
- **SamlConfigStateService**
- **ScmEventBusStateService**
- **SentinelConfigStateService**
- **ServiceConfigStateService**
- **ServiceHealthStateService**
- **Session**
- **SmbCertService**
- **StratumConfigStateService**
- **StratumQuery**
- **TenantAdmin**
- **TimeDbStateService**
- **TimedbConfigStateService**
- **TimedbQuery**
- **TimeseriesMutation**
- **TlsCertService**
- **TraceEventBusStateService**
- **TraceSession**
- **TrustedCaStateService**
- **WorkloadStateService**

## Packages

- `clients/cavalier`
- `clients/cavalierctl`
- `common/credentials`
- `common/hazelcast`
- `common/tokenrequest`
- `generated/accounting_method`
- `generated/entitlements`
- `generated/project_config`
- `generated/services`
- `generated/services_config`
- `generated/tenant_config`
- `libs/accounting`
- `libs/alpn`
- `libs/cloudidentity`
- `libs/cloudvendor`
- `libs/credpair`
- `libs/diskstore`
- `libs/errors`
- `libs/filesystem`
- `libs/gatewayjwksview`
- `libs/graph_api`
- `libs/grpcconfig`
- `libs/healthreport`
- `libs/host`
- `libs/http`
- `libs/httpclient`
- `libs/ip`
- `libs/jwk`
- `libs/jwkview`
- `libs/key`
- `libs/lock`
- `libs/logger`
- `libs/opalib`
- `libs/outputformat`
- `libs/query`
- `libs/saml`
- `libs/smbcert`
- `libs/spec`
- `libs/tacplus`
- `libs/tlscert`
- `libs/token`
- `libs/tokencache`
- `libs/trace`
- `libs/x509`
- `services/anthropic`
- `services/argus`
- `services/eventbus`
- `services/gateway`
- `services/github`
- `services/gitlab`
- `services/gitlab-connector`
- `services/graphdb`
- `services/kernel`
- `services/kmaker`
- `services/letsencrypt`
- `services/mcp`
- `services/msggateway`
- `services/oauth`
- `services/opa`
- `services/openai`
- `services/operator`
- `services/saml`
- `services/sentinel`
- `services/stratum`
- `services/timedb`
- `vservices/claude-sensor`
- `vservices/entitlements`
- `vservices/gitlab-connector`
- `vservices/llm-sensor`
- `vservices/proxy-policy`
- `vservices/scm-sensor`

## Documentation

Additional documentation is available in the `docs` map at the root of this registry.
