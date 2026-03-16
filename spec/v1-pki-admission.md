# Transitloom v1 PKI and Admission Specification

## Status

Draft

This document specifies the Transitloom v1 PKI, node identity, coordinator intermediate model, admission-token model, revocation behavior, renewal behavior, and the relationship between trust and current participation permission.

This document focuses on **identity and authorization to participate**, not on the full control-plane message protocol or the raw UDP data plane.

---

## 1. Purpose

Transitloom v1 separates:

- **who a node is**
- **whether that node is currently allowed to participate**

This specification exists to define that separation clearly.

Transitloom requires this separation because:

- long-lived identity is operationally useful
- hard revoke is operationally necessary
- admission state must be enforceable even if an identity certificate is still valid
- coordinator-managed overlay participation should not depend only on certificate lifetime

The v1 model therefore uses:

- **PKI-backed identity**
- **short-lived admission tokens**

---

## 2. v1 design goals

### 2.1 Primary goals

- Provide a private PKI under Transitloom control
- Support a single root trust anchor
- Support per-coordinator intermediate CAs
- Support node identity certificates
- Support hard revoke without requiring extremely short-lived node certificates
- Support renewal through authorized coordinators
- Support relay-assisted renewal when policy allows
- Keep the model operationally manageable for admin-operated coordinators

### 2.2 Secondary goals

- Avoid dependence on public Internet PKI
- Avoid external OpenSSL CLI or system OpenSSL requirements
- Support overlapping intermediate lifecycle management
- Support multi-coordinator HA for issuance
- Support deterministic operational monitoring for certificate and authority lifecycles

---

## 3. Non-goals for v1

Transitloom v1 PKI/admission does not aim to provide:

- public web PKI compatibility
- external ACME integration
- OCSP/CRL-heavy Internet-style PKI dependencies
- anonymous node participation
- certificate-only authorization without admission-state checks
- root-authority exposure as a normal end-user coordinator service

---

## 4. Core model

Transitloom v1 uses four distinct trust/authorization objects:

- **Root CA**
- **Coordinator Intermediate CA**
- **Node Certificate**
- **Admission Token**

These objects have different roles and must not be confused.

---

## 5. Trust roles

## 5.1 Root CA

The Root CA is the trust anchor for the Transitloom deployment.

Its responsibilities include:

- anchoring trust for the Transitloom coordinator network
- issuing or authorizing coordinator intermediate CA certificates
- supporting intermediate lifecycle events
- remaining outside the normal end-user coordinator path

### v1 requirements
- The Root CA is not a normal node-facing coordinator target
- The Root CA should not serve ordinary coordinator traffic to end-user nodes
- The Root CA is logically separate from normal coordinators, even if code or infrastructure is shared

## 5.2 Coordinator Intermediate CA

Each authorized coordinator may have its own intermediate CA certificate chained to the Transitloom Root CA.

Its responsibilities include:

- issuing node identity certificates
- renewing node identity certificates
- participating in issuance under current policy
- proving that the node identity chains back to the Transitloom root

### v1 requirements
- Coordinator intermediates are the normal issuing authorities for node certificates
- Node certificate issuance should not require the root to be online for routine renewals
- Coordinators must not issue outside the permissions of the Transitloom deployment

## 5.3 Node Certificate

A node certificate identifies a node.

A node certificate is not by itself proof of current permission to participate.

It exists to provide:

- stable node identity
- cryptographic authentication material for mTLS
- a chain of trust to the Transitloom root

## 5.4 Admission Token

An admission token is a short-lived signed authorization proving that the node is currently allowed to participate in the Transitloom network.

The admission token is distinct from the node certificate.

It exists to provide:

- current participation permission
- hard revoke enforcement
- rapid operational control without forcing extremely short certificate lifetimes

---

## 6. Identity vs participation permission

This distinction is fundamental.

## 6.1 Identity

Identity answers:

- Which node is this?

Identity is represented by the node certificate and corresponding key material.

## 6.2 Participation permission

Participation permission answers:

- Is this node currently allowed to participate?

Participation permission is represented by:

- current global admission state
- a valid short-lived admission token

## 6.3 Mandatory v1 rule

Normal node participation requires both:

- a valid node certificate
- a valid admission token

A certificate alone is not sufficient.

---

## 7. Node identity model

## 7.1 Node identity anchor

A node identity is anchored to node-generated or node-held key material and its corresponding certificate chain.

Transitloom v1 expects node identity to survive ordinary restarts and survive reinstall/redeploy only if the node’s identity material survives.

## 7.2 Identity persistence

If the node private key and relevant persisted identity state survive, the node may preserve identity across reinstall or redeploy.

If the identity material does not survive, the node should be treated as a new identity unless a future recovery mechanism is added.

## 7.3 Identity continuity rule

Transitloom v1 should not pretend that reinstall without preserved identity material is the same node identity.

---

## 8. Admission lifecycle

Transitloom v1 must support at least these conceptual admission states:

- **pending**
- **admitted**
- **revoked**

The exact schema may include versioning and metadata, but these concepts are required.

## 8.1 Pending

A node is known or enrolling but not yet admitted for normal participation.

## 8.2 Admitted

A node is currently allowed to participate and may obtain or use valid admission tokens and normal control sessions.

## 8.3 Revoked

A node is not currently allowed to participate.

Revoked means:

- normal control sessions must be rejected
- reconnect attempts must fail
- admission-token issuance must not proceed
- certificate renewal must not proceed as normal participation

## 8.4 Re-admission

Re-admission is a fresh authorization event.

Transitloom v1 treats re-admission as requiring approval again, rather than automatically restoring prior transient participation state.

---

## 9. Hard revoke semantics

Transitloom v1 uses **hard revoke** semantics.

## 9.1 Meaning

A revoked node must not be able to continue normal participation merely because its identity certificate has not yet expired.

## 9.2 Enforcement

Hard revoke is enforced by:

- current admission-state checks
- admission-token validity checks
- coordinator-side session rejection

## 9.3 Consequence

A still-valid node certificate does not override a revoked admission state.

---

## 10. Node certificates

## 10.1 Purpose

Node certificates exist to identify nodes and participate in mTLS control sessions.

## 10.2 Suggested contents

The exact certificate encoding is implementation-defined, but a node certificate should be able to represent at least:

- stable node identity
- appropriate key usage / extended usage for Transitloom node identity
- issuer chain to a valid coordinator intermediate and root

## 10.3 Lifetime

Transitloom v1 may use medium-to-long node certificate lifetimes for operational practicality.

Longer-lived certificates are acceptable because current participation permission is enforced separately through admission tokens and current admission state.

## 10.4 Renewal

Node certificate renewal may be performed by any authorized coordinator intermediate according to policy.

A node should not be required to renew only from the same coordinator that issued its previous certificate.

---

## 11. Coordinator intermediates

## 11.1 Purpose

Coordinator intermediates allow routine issuance to be performed by coordinators without requiring the root authority to be online for every node renewal.

## 11.2 Lifecycle

Coordinator intermediates must support:

- issuance by the root authority
- renewal in advance of expiry
- overlap with replacement intermediates
- operational monitoring of remaining validity

## 11.3 Overlapping issuance windows

Transitloom v1 prefers overlapping intermediate lifecycle windows so that new intermediate credentials can be prepared before old ones expire.

This reduces root-authority availability pressure.

## 11.4 Root independence for ordinary node renewals

Routine node certificate renewal should depend on a valid coordinator intermediate, not on the root being online at the time of the renewal.

---

## 12. Root authority lifecycle

## 12.1 Role

The root authority should be used for:

- initial trust anchor creation
- coordinator intermediate issuance
- intermediate replacement/rotation
- root rollover or recovery operations

## 12.2 Not required for routine node renewals

The root authority should not be required to be online for ordinary node certificate renewal if coordinators already have valid intermediates.

## 12.3 Operational model

The root authority may be:

- a dedicated special service
- an isolated administrative mode
- another carefully controlled deployment pattern

But it must remain outside normal end-user coordinator exposure.

---

## 13. Admission tokens

## 13.1 Purpose

Admission tokens provide short-lived proof that a node is currently authorized to participate.

## 13.2 Why tokens exist

Transitloom uses admission tokens to avoid overloading the certificate with current authorization semantics.

This makes it possible to have:

- longer-lived node certificates
- faster and cleaner revoke behavior
- coordinator-side participation enforcement
- lower operational dependency on very short certificate renewals

## 13.3 Required semantics

A valid admission token must mean:

- the node is currently admitted
- the token is within its valid lifetime
- the token is accepted under current policy
- the node is not globally revoked

## 13.4 Mandatory usage rule

Coordinators must require a valid admission token for normal node participation.

This includes normal control session establishment.

---

## 14. Admission-token issuance

## 14.1 Issuers

Admission tokens are issued by authorized coordinator-side logic according to current global admission state and policy.

## 14.2 Preconditions

A node may only obtain a valid admission token if:

- it has a valid identity chain
- it is currently admitted
- it is not revoked
- policy permits token issuance

## 14.3 Denial cases

Admission-token issuance must be denied when:

- the node is pending but not yet admitted
- the node is revoked
- trust state is invalid
- issuance policy disallows the operation

## 14.4 Renewal and refresh

Because admission tokens are intentionally short-lived, nodes must refresh them during normal operation.

The exact token lifetime and refresh rules are implementation-specific and should be defined in future lower-level specs.

---

## 15. Participation checks at coordinators

## 15.1 Normal session establishment rule

A coordinator must verify all of the following for a normal node control session:

- valid certificate chain
- acceptable node identity
- valid admission token
- current admission state is admitted
- current admission state is not revoked

## 15.2 Certificate alone is insufficient

A coordinator must not accept a normal node session purely because a certificate is still valid.

## 15.3 Revoked node rule

A revoked node must be rejected even if:

- its node certificate remains valid
- its old transport path is still reachable
- it attempts reconnect via a different coordinator

---

## 16. Renewal behavior

## 16.1 Node certificate renewal

Node certificate renewal may occur through any authorized coordinator intermediate according to policy.

## 16.2 Relay-assisted renewal

Relay-assisted coordinator access may be used for renewal when policy allows.

This is important for nodes behind difficult network conditions.

## 16.3 Renewal is not admission

Certificate renewal and admission-token issuance are distinct operations.

A node may have its identity certificate validity handled separately from whether it is currently admitted.

## 16.4 Revoked node renewal

A revoked node must not be allowed to continue ordinary participation through normal renewal flows.

Any exceptional recovery process would need to be explicitly defined and is not assumed by v1.

---

## 17. Coordinator partition behavior for security-sensitive state

Transitloom v1 must distinguish committed security-sensitive state from pending proposals.

## 17.1 Pending-only acceptance

If a coordinator is partitioned from the rest of the coordinator network, it may accept a security-sensitive administrative action only as a **pending proposal**.

Examples include:

- admit node
- revoke node
- trust changes

## 17.2 No node action on pending proposals

Nodes must not treat pending proposals as committed truth.

## 17.3 Ordered operation model

Security-sensitive state changes should be represented as ordered operations rather than weak overwrite semantics.

This applies especially to:

- admit
- revoke
- trust changes

The exact operation-log structure is specified elsewhere, but the PKI/admission model depends on ordered handling.

---

## 18. Re-admission behavior

## 18.1 Fresh authorization

If a revoked node is re-admitted, that is a fresh authorization event.

## 18.2 Not automatic restoration

Re-admission must not be treated as automatic restoration of all prior transient state.

## 18.3 Result

After re-admission, the node may once again obtain:

- valid admission tokens
- normal control session acceptance
- renewal eligibility under policy

subject to the normal issuance and control rules.

---

## 19. Root and coordinator discovery behavior

## 19.1 Root authority visibility

The root authority should not be visible in normal coordinator discovery to end-user nodes as a standard usable coordinator target.

## 19.2 Coordinator discovery

Nodes may discover normal coordinators through authorized discovery processes.

## 19.3 Trust source rule

Node-provided discovery information is treated as a hint, not as trust truth.

Trust still depends on the valid chain to the Transitloom root and current policy.

---

## 20. Suggested lifecycle categories

This section gives the conceptual lifecycle categories that the implementation must support.

## 20.1 Root CA lifecycle
- created
- active
- replacement planned
- rolled over or retired

## 20.2 Coordinator intermediate lifecycle
- issued
- active
- renewal window open
- overlap with replacement
- retired

## 20.3 Node certificate lifecycle
- requested
- issued
- active
- renewal eligible
- expired
- superseded

## 20.4 Admission token lifecycle
- issued
- active
- refresh due
- expired
- invalid due to revoke or admission-state change

---

## 21. Operational guidance

## 21.1 Long-lived certs, short-lived tokens

Transitloom v1 should prefer:

- longer-lived node certificates for identity stability
- shorter-lived admission tokens for current authorization

## 21.2 Intermediate overlap

Coordinators should renew or replace intermediates early enough that root availability is not needed at the last moment.

## 21.3 Monitoring

The implementation should expose useful operational visibility for:

- root validity horizon
- coordinator intermediate validity horizon
- node certificate validity horizon
- admission-token freshness
- revoke state
- current admission eligibility

---

## 22. Security properties intended by the model

The Transitloom v1 PKI/admission model aims to provide:

- stable authenticated node identity
- centrally controlled current participation permission
- hard revoke semantics
- reduced operational pressure from very short identity certificates
- multi-coordinator issuance flexibility
- coordinator-managed trust enforcement

It intentionally does not aim to make the certificate itself the only source of authorization truth.

---

## 23. v1 constraints summary

Transitloom v1 PKI/admission includes:

- one root authority
- per-coordinator intermediates
- node identity certificates
- short-lived mandatory admission tokens
- hard revoke behavior
- relay-assisted renewal where policy allows
- pending-only handling of partitioned security-sensitive admin writes
- ordered security-sensitive operation semantics

Transitloom v1 PKI/admission does not include:

- certificate-only authorization
- root authority as a normal node-facing coordinator
- public Internet PKI dependencies
- dependence on the root for routine node renewals
- assumption that still-valid certificates override current revoke state

---

## 24. Deferred and future work

The following items are intentionally left for later or for more detailed documents:

- exact certificate field encoding
- exact token format and signing rules
- token lifetime values
- intermediate lifetime values
- root rollover ceremony details
- explicit emergency recovery flows
- detailed audit/event model
- admin CLI/API semantics for trust operations

---

## 25. Related specifications

This PKI/admission specification depends on and should remain consistent with:

- `spec/v1-architecture.md`
- `spec/v1-control-plane.md`
- `spec/v1-data-plane.md`
- `spec/v1-service-model.md`
- `spec/v1-wireguard-over-mesh.md`

---
