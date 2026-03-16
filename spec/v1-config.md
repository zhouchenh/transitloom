# Transitloom v1 Configuration Specification

## Status

Draft

This document defines the Transitloom v1 configuration model. It describes the intended configuration structure for the root authority, coordinators, nodes, services, local ingress bindings, bootstrap coordinators, policy references, and persistence-related settings.

This document is a configuration specification, not a final file-format lock. It defines the conceptual configuration model and the configuration fields Transitloom v1 should be able to express, while leaving room for implementation details such as exact YAML/TOML/JSON encoding.

---

## 1. Purpose

Transitloom has enough architectural shape that configuration must now become explicit.

This specification exists to answer questions such as:

- What must be configured on a root authority?
- What must be configured on a coordinator?
- What must be configured on a node?
- How are services declared?
- How are WireGuard services represented?
- How are bootstrap coordinators configured?
- What is static config versus persisted runtime state?
- Which settings are local-only versus coordinator-managed?

The goal is to ensure the implementation has a stable and disciplined configuration model before code grows.

---

## 2. Design goals

### 2.1 Primary goals

- Provide a clear and minimal v1 configuration model
- Separate static config from persisted/generated runtime state
- Support root, coordinator, node, and service roles cleanly
- Support the flagship WireGuard-over-mesh use case directly
- Support stable local ingress bindings
- Keep the config model generic rather than WireGuard-only
- Allow coordinator-managed policy to coexist with local config

### 2.2 Secondary goals

- Leave room for future transport/service modes
- Avoid forcing every future detail into static config
- Make configuration understandable to operators
- Support deterministic behavior where applications depend on stable local bindings

---

## 3. Non-goals for v1

Transitloom v1 configuration does not aim to provide:

- a fully finalized external API contract
- every future routing/policy feature in static config
- a giant all-in-one config surface for every eventual capability
- a requirement that every piece of runtime state be manually configured
- protocol-specific config behavior for every possible application

---

## 4. Configuration model overview

Transitloom v1 has four major configuration scopes:

- **root authority config**
- **coordinator config**
- **node config**
- **service config**

It also distinguishes between:

- **operator-authored configuration**
- **persisted runtime state**
- **coordinator-managed distributed state**

These must not be confused.

---

## 5. Static config vs runtime state vs distributed state

## 5.1 Static config

Static config is operator-authored local configuration.

Examples:
- bind addresses
- data directories
- enabled role flags
- bootstrap coordinators
- local service declarations
- local policy toggles
- local private-address advertisement choices

## 5.2 Persisted runtime state

Persisted runtime state is local state created and maintained by Transitloom.

Examples:
- locally generated keys
- locally assigned stable ingress bindings
- cached certificates
- cached admission tokens
- persisted service IDs if needed
- node identity material
- coordinator-issued intermediate material

## 5.3 Distributed state

Distributed state is coordinator-managed or globally meaningful state, not primarily operator-authored local config.

Examples:
- admission state
- revoke state
- ordered global operations
- service registry records
- association objects
- distributed policy state

### Important rule

Transitloom should not force operators to manually encode distributed state in every local config file.

---

## 6. Configuration file strategy

Transitloom v1 should support a local config file per runtime role.

Examples:
- root authority config file
- coordinator config file
- node config file

The exact serialization format may be chosen later, but the model should be compatible with a structured text format such as YAML or TOML.

### v1 recommendation

Prefer one local structured config file per process role, plus a data directory for persisted runtime state.

---

## 7. Common configuration concepts

The following concepts appear across multiple config scopes.

## 7.1 Identity metadata

Human-readable metadata such as:
- name
- label
- description
- tags

## 7.2 Network endpoints

Addresses and ports for:
- control transport listeners
- relay listeners
- local service bindings
- local ingress bindings

## 7.3 Storage paths

Filesystem locations for:
- keys
- certificates
- token cache
- runtime DB/state
- logs where applicable

## 7.4 Policy references

References to policy objects or policy labels, where local config must attach to higher-level policy.

## 7.5 Enable/disable controls

Role-local switches for:
- enabling a listener
- enabling relay capability
- enabling service advertisement
- enabling private-address sharing
- enabling coordinator discovery participation

---

## 8. Root authority configuration

The root authority is a special trust role and should have its own configuration scope.

## 8.1 Root authority config responsibilities

Root config should cover:

- root identity metadata
- trust material paths or generation mode
- storage locations
- issuance policy for coordinator intermediates
- administrative bind/listen settings if applicable
- security posture defaults
- lifecycle management settings

## 8.2 Conceptual root config fields

A root authority config should be able to express at least:

- root name / label
- data directory
- root key path or key generation mode
- root certificate path
- admin API / control listen address if applicable
- issuance policy defaults
- logging/status settings
- allowed admin identities or admin auth source references
- backup/export/import settings where applicable

## 8.3 Root authority visibility rule

Root config must not expose the root as a normal node-facing coordinator target.

The root is a trust role, not a normal public coordinator service for end-user nodes.

---

## 9. Coordinator configuration

A coordinator is the main node-facing control-plane infrastructure role.

## 9.1 Coordinator config responsibilities

Coordinator config should cover:

- coordinator identity metadata
- control transport listeners
- relay listener configuration
- data directory
- trust material references
- intermediate issuance/renewal behavior
- discovery visibility settings
- local serving-policy settings
- bootstrap/peer coordinators
- relay capability and limits
- policy attachment defaults
- observability settings

## 9.2 Conceptual coordinator config fields

A coordinator config should be able to express at least:

- coordinator name / label
- coordinator ID override or persisted identity policy
- data directory
- control listen endpoints
- relay listen endpoints
- QUIC enable/disable and bind settings
- TCP fallback enable/disable and bind settings
- root trust anchor path/reference
- coordinator intermediate key/cert path/reference
- peer coordinator bootstrap list
- relay enabled flags
- relay limits and capacity-related local settings
- discovery visibility settings
- local serving-policy defaults
- logging / metrics / status settings

## 9.3 Peer coordinators

Coordinator config should allow specifying peer coordinators for:
- control-network bootstrap
- distributed state participation
- discovery seeding

This is not the same thing as exposing those peers to end-user nodes for ordinary discovery.

## 9.4 Local serving policy hooks

Coordinator config should allow local serving-policy behavior such as:
- concealment-like behavior
- local drain mode
- local relay disable/enable
- local maintenance restrictions

These are coordinator-local operational settings, not global trust-state replacements.

---

## 10. Node configuration

Node config is the main operator-authored config for a Transitloom participant that exposes services and carries traffic.

## 10.1 Node config responsibilities

Node config should cover:

- local identity metadata
- data directory
- bootstrap coordinators
- local control transport preferences
- local service declarations
- local private-address sharing preferences
- local relay capability preferences
- local path metering metadata where needed
- observability settings

## 10.2 Conceptual node config fields

A node config should be able to express at least:

- node name / label
- data directory
- control transport preferences
- bootstrap coordinator list
- local service list
- local discovery/advertisement toggles
- local private-address sharing toggles
- local relay participation toggles
- local policy labels / group labels
- logging / metrics / status settings

## 10.3 Node identity material

Node config should not require operators to paste raw identity material inline by default.

Instead, node config should refer to:
- persisted key/cert paths
- or a data directory where Transitloom manages local identity state

## 10.4 Bootstrap coordinator list

A node config should allow one or more bootstrap coordinators.

Each bootstrap coordinator entry should be able to express at least:
- coordinator ID hint if known
- one or more control transport endpoints
- transport preferences
- optional trust metadata hints
- optional label/region metadata

Bootstrap coordinator entries are bootstrap hints, not final trust truth.

---

## 11. Service declaration model in config

Services are first-class and must be configurable locally on nodes.

## 11.1 Purpose

Service declarations tell Transitloom:
- what local services exist
- how they are bound locally
- how they should be advertised or exposed
- what local ingress behavior is needed

## 11.2 Service declaration requirements

A service declaration should be able to express at least:

- service name
- service type
- service identity policy
- local target binding
- discoverability intent
- relay/path preferences
- capability declarations
- service-level policy labels
- optional local-ingress strategy

## 11.3 Service type support in v1

The primary v1 service type is:
- raw UDP

But the config model may allow future capability declarations, such as future TCP support, without requiring those transports to exist yet.

---

## 12. Service binding configuration

A service must map to a concrete local endpoint.

## 12.1 Local target binding

For raw UDP services, config must be able to express:

- local target address
- local target port
- address family preference or explicit bind family where needed

Examples:
- `127.0.0.1:51820`
- `[::1]:51820`
- another explicitly allowed local address/port

## 12.2 Binding constraints

The config model should make it clear that:
- service local target is for inbound delivery to the application/service
- this is not the same thing as a local ingress port used by the application to send to remote services

---

## 13. Local ingress configuration

Transitloom v1 must support stable local ingress behavior for application-facing use cases such as WireGuard-over-mesh.

## 13.1 Purpose

A local ingress binding is a local endpoint Transitloom listens on so that a local application can send traffic into the mesh toward a remote service association.

## 13.2 Config responsibilities

Node/service config should be able to express at least one of these strategies:

- explicit static local ingress assignment
- deterministic local ingress allocation from a configured range
- persisted auto-assignment with stable recovery across restarts

## 13.3 v1 recommendation

Transitloom v1 should default to deterministic and stable local ingress bindings where application config depends on them.

## 13.4 Config fields for ingress strategy

A node or service config should be able to express fields such as:

- ingress mode
- ingress base range or reserved range
- deterministic allocation enabled/disabled
- explicit static overrides
- loopback family preference (`127.0.0.1` vs `::1` or equivalent behavior)

---

## 14. WireGuard service configuration

WireGuard is the flagship v1 use case, so the config model must represent it cleanly.

## 14.1 Core principle

WireGuard remains standard. Transitloom config should describe the Transitloom side of the integration, not redefine WireGuard itself.

## 14.2 Required WireGuard-related service config fields

A WireGuard service declaration should be able to express at least:

- service name
- service type indicating UDP carriage use
- local target address/port corresponding to the WireGuard `ListenPort`
- local ingress behavior for peer associations
- optional service labels
- optional relay/path preferences
- optional discovery controls

## 14.3 Important distinction

The WireGuard local `ListenPort` is the local target for inbound delivery.

Transitloom local ingress ports used as WireGuard peer endpoints are separate and should be represented separately or via ingress allocation policy.

## 14.4 Future tooling note

Transitloom v1 config does not require automatic WireGuard config rewriting, but the config model should be compatible with future helper tooling or snippet generation.

---

## 15. Bootstrap coordinator configuration

Bootstrap coordinators are required so nodes can join the coordinator network.

## 15.1 Node-side bootstrap entries

A bootstrap coordinator entry should be able to express:

- label or name
- one or more control endpoints
- allowed transport types (QUIC, TCP)
- transport preference
- optional coordinator ID hint
- optional expected trust anchor reference hint
- optional region or tag metadata

## 15.2 Coordinator-side peer bootstrap entries

A coordinator config should likewise support bootstrap/peer entries for other coordinators, with similar fields.

## 15.3 Trust rule

Bootstrap configuration provides connection hints, not trust truth.

Actual trust still depends on:
- valid certificate chain
- correct root anchor
- current policy

---

## 16. Discovery-related local config

Discovery is partly coordinator-managed, but local config still needs some control hooks.

## 16.1 Node-side discovery-related fields

A node config should be able to control things such as:

- whether coordinator discovery is enabled
- whether node-provided discovery hints may be shared
- whether private addresses may be advertised
- whether service discoverability is enabled by default
- whether specific services are hidden locally

## 16.2 Coordinator-side discovery-related fields

A coordinator config should be able to control things such as:

- whether the coordinator is discoverable
- which transport endpoints are advertised
- whether it participates in catalogs
- local discovery visibility policy hooks

---

## 17. Relay-related local config

Transitloom v1 distinguishes control relay and data relay. Config must be able to express that distinction.

## 17.1 Coordinator relay config

Coordinator config should be able to express:

- control relay enabled/disabled
- data relay enabled/disabled
- relay listen endpoints
- relay limits
- optional per-role relay restrictions
- local drain or maintenance behavior

## 17.2 Node relay config

Node config should be able to express:

- control relay capability enabled/disabled
- data relay capability enabled/disabled
- local relay participation preferences
- optional local limits
- discovery visibility of relay capability
- local policy labels attached to relay behavior

## 17.3 v1 caution

Even if data relay is allowed by policy, config should make it easy to restrict or disable it per node/coordinator.

---

## 18. Path and metering hints in config

Transitloom should avoid forcing every live path decision into static config, but local config still needs to express useful hints.

## 18.1 Metering

Because metering is a per-path concept, config should allow operators to express local path or interface-level hints where possible.

Examples of configurable hint types:
- metered
- unmetered
- avoid active capacity testing
- prefer for control only
- never use for relay
- local cost/weight hints

## 18.2 Local interface/path hint model

Exact per-interface or per-address binding shape may be refined later, but v1 config should leave room for local path metadata attachment.

---

## 19. Policy references in config

Transitloom has both distributed policy and local config. Local config should be able to reference higher-level policy without hardcoding all policy inline.

## 19.1 Examples

Config should be able to carry references such as:
- node group labels
- service policy labels
- relay policy labels
- discovery policy labels

## 19.2 Important distinction

Local config references policy; it is not necessarily the complete source of truth for distributed policy.

---

## 20. Persistence and state directories

Transitloom relies on persisted state for stable operation.

## 20.1 Required persistence categories

Each role should have a data directory or equivalent state location for things such as:
- generated keys
- issued certificates
- cached tokens
- stable ingress assignments
- local IDs
- local databases/state
- cached coordinator/service/association data as appropriate

## 20.2 v1 recommendation

Each runtime role should have:
- one primary data directory
- optional separate log directory if desired
- local file/path references derived relative to the data directory when possible

## 20.3 Operator expectation

Operators should configure directories and major paths, but Transitloom should manage most runtime-generated contents inside those directories.

---

## 21. Logging, metrics, and status config

Config should expose a consistent observability surface.

## 21.1 Logging fields

At minimum, config should support:
- log level
- log format
- log destination

## 21.2 Metrics/status fields

At minimum, config should support:
- metrics enabled/disabled
- metrics listen address if applicable
- status/health endpoint enabled/disabled
- status bind address if applicable

## 21.3 Debug caution

Verbose debugging should be configurable but should not be mandatory for normal operation.

---

## 22. Recommended top-level config sections

A role-specific config file should likely organize fields into sections such as:

- identity / metadata
- storage
- control transport
- trust
- discovery
- relay
- services
- local ingress
- observability
- policy references

Not every role uses every section, but this gives a consistent mental model.

---

## 23. Role-specific config summary

## 23.1 Root authority config should include

- identity metadata
- storage paths
- trust material paths or generation mode
- issuance settings
- admin control endpoint if applicable
- observability

## 23.2 Coordinator config should include

- identity metadata
- storage paths
- control listeners
- relay listeners
- root trust anchor reference
- intermediate reference
- peer coordinators
- discovery visibility settings
- relay settings
- local serving-policy hooks
- observability

## 23.3 Node config should include

- identity metadata
- storage paths
- bootstrap coordinators
- transport preferences
- service declarations
- local ingress policy/allocation settings
- local discovery/private-address settings
- relay participation settings
- observability

---

## 24. Example conceptual config shapes

The exact syntax is not yet locked, but the following conceptual shapes show what v1 config should be able to represent.

## 24.1 Root authority conceptual shape

A root authority config should be able to represent concepts like:

- root name
- data directory
- root key location
- root cert location
- issuance defaults
- admin bind
- observability settings

## 24.2 Coordinator conceptual shape

A coordinator config should be able to represent concepts like:

- coordinator name
- data directory
- control listeners
- relay listeners
- root trust anchor path
- intermediate key/cert path
- peer coordinators
- discoverable yes/no
- relay enabled flags
- observability settings

## 24.3 Node conceptual shape

A node config should be able to represent concepts like:

- node name
- data directory
- bootstrap coordinators
- services list
- ingress allocation strategy
- private-address sharing allowed yes/no
- relay participation flags
- observability settings

## 24.4 WireGuard service conceptual shape

A WireGuard-oriented service config should be able to represent concepts like:

- service name
- service type = UDP-carried service
- local target = WireGuard local listen port
- ingress strategy = deterministic stable local ingress
- service labels
- discovery controls
- relay/path preferences if needed

---

## 25. Validation expectations

Transitloom config should be validated before role startup proceeds.

## 25.1 Validation categories

Validation should cover at least:

- required fields present
- mutually exclusive settings rejected
- illegal role combinations rejected where appropriate
- local endpoints parse correctly
- storage paths are acceptable
- impossible ingress strategy combinations rejected
- relay/data role constraints enforced
- trust references consistent enough for startup

## 25.2 Strictness recommendation

Prefer stricter startup validation over ambiguous runtime guessing.

---

## 26. Relationship to the object model

Config is not the whole object model.

### Config provides:
- local intent
- local static settings
- bootstrap information
- service declarations
- local runtime constraints

### Distributed state provides:
- admission state
- service registry truth
- association distribution
- relay/path legality
- global operations

### Persisted runtime state provides:
- keys
- certs
- tokens
- stable ingress assignments
- local caches

This separation is critical.

---

## 27. v1 constraints summary

Transitloom v1 config should support:

- separate root/coordinator/node role configs
- local service declarations
- stable local ingress behavior
- bootstrap coordinator lists
- relay role controls
- local discovery controls
- persistence directories
- observability settings
- policy references

Transitloom v1 config should not require:

- manual encoding of all distributed state
- inline storage of every secret in the main config
- protocol-specific special casing that breaks the generic service model
- arbitrary future routing behavior to be preconfigured in static config

---

## 28. Deferred and future work

The following items are intentionally deferred:

- finalized on-disk syntax examples
- environment-variable override strategy
- hot-reload semantics
- full admin API config surface
- future TCP transport config details
- future encrypted data-plane config details
- advanced policy templating

These can be refined later without changing the conceptual v1 config model.

---

## 29. Related specifications

This configuration specification depends on and should remain consistent with:

- `spec/v1-architecture.md`
- `spec/v1-control-plane.md`
- `spec/v1-data-plane.md`
- `spec/v1-service-model.md`
- `spec/v1-pki-admission.md`
- `spec/v1-wireguard-over-mesh.md`
- `spec/v1-object-model.md`
- `spec/implementation-plan-v1.md`

---
