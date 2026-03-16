# agents/SOUL.md

## Purpose

This file captures the design philosophy of Transitloom.

It exists to answer questions like:

- how should agents make tradeoffs?
- what should be protected even when implementation pressure rises?
- what kinds of “good ideas” are actually bad for this project?
- what values should shape decisions when the specs do not answer every detail directly?

This file is not a protocol spec.  
It is the project’s decision compass.

---

## The soul of Transitloom

Transitloom is being built for **real networks**, not idealized ones.

Its soul is a combination of:

- practical transport engineering
- architectural discipline
- strong control over participation
- respect for messy underlays
- performance-first thinking for the flagship use case
- generic foundations without pretending every future feature belongs in v1

Transitloom should feel like a transport fabric that is:

- grounded
- explicit
- disciplined
- useful
- not fragile
- not over-theoretical

---

## The primary truth

The primary truth of Transitloom is:

**the first thing that must work well is high-performance raw UDP overlay transport for real deployments**

Everything else is secondary to that.

Not because other things do not matter, but because they only matter if the transport foundation is real.

If Transitloom gets:
- trust right
- admission right
- service model right
- local ingress model right
- direct/relay model right
- scheduling right

then it can grow.

If it gets those wrong, no amount of extra feature surface will save it.

---

## The first practical target

Transitloom’s first practical target is not “generic future elegance.”

It is:

**WireGuard-over-mesh with meaningful multi-WAN raw UDP aggregation value**

That target should guide decisions.

When agents are unsure between:
- a broad abstract framework
- and a narrower path that makes the flagship use case work sooner and better

the narrower path usually wins.

---

## The most important tradeoff rule

When tradeoffs become difficult, Transitloom should generally prefer:

- **performance over unnecessary routing freedom**
- **clarity over cleverness**
- **explicit state over hidden magic**
- **maintainability over abstraction vanity**
- **generic core over protocol-specific hacks**
- **real operational control over optimistic assumptions**
- **end-to-end usefulness over locally elegant machinery**

This rule should shape implementation decisions, not just documentation.

---

## Real networks first

Transitloom is not being built for neat diagrams.

It is being built for environments where:
- addresses change
- relays matter
- intranet paths appear and disappear
- NAT/CGNAT complicates direct connectivity
- some links are good, some are expensive, some are unstable
- operators still need predictability

That means Transitloom must respect operational reality.

Agents should not implement as though:
- one path is always stable
- direct is always available
- relay is always bad
- relay is always fine
- one layer can “just infer” the rest
- the underlay will stay nice enough to reward sloppy assumptions

---

## Generic core, concrete flagship

Transitloom intentionally holds two truths at once:

### Truth 1
The core model should remain generic.

### Truth 2
The flagship use case should shape v1 priorities.

These are not contradictions.

The generic core means:
- services stay generic
- associations stay generic
- bindings stay generic
- control-plane concepts stay generic

The flagship focus means:
- docs and examples optimize around WireGuard
- data-plane choices should help raw UDP aggregation
- local ingress design should support real WireGuard deployment
- performance decisions should be made with the flagship use case in mind

Agents must preserve both truths at the same time.

---

## Scope discipline is part of the soul

Transitloom is not a “say yes to every interesting overlay idea” project.

A design can be:
- intelligent
- elegant
- ambitious

and still be wrong for v1.

The soul of Transitloom includes restraint.

That means:
- no arbitrary multi-hop raw UDP data forwarding in v1
- no speculative transport layers pretending to be real already
- no broad service-mesh ambitions before the flagship transport path works
- no premature admin-UX polish before transport correctness exists
- no architectural drift caused by implementation convenience

Saying “not yet” is part of building the right thing.

---

## Identity and admission are sacred

Transitloom must not blur the line between:

- who a node is
- whether a node is currently allowed to participate

That separation is one of the most important design choices in the project.

Why it matters:
- certificates can be medium/long-lived for identity stability
- admission tokens can be short-lived for hard revoke behavior
- current participation can be enforced operationally
- trust and authorization remain explicit instead of being muddled together

Agents must protect this separation.

If implementation convenience pressures the code toward “cert means fully allowed,” that is architectural regression.

---

## Local target and local ingress are sacred

Transitloom must not blur the line between:

- where inbound traffic is delivered for a service
- where a local application sends traffic into the mesh

This distinction is especially important for WireGuard-over-mesh.

It is tempting to simplify these concepts into one port or one socket model.

That temptation should usually be resisted.

Why:
- the service model becomes cleaner when these are separate
- WireGuard mapping stays practical
- future UDP services benefit from the same structure
- the architecture remains generic instead of application-bound

Agents must preserve this distinction.

---

## Endpoint-owned scheduling is sacred

The endpoint association owns the end-to-end split decision for v1 data plane.

Relays do not get to become free-willed traffic optimizers.

Why:
- per-hop independence quickly becomes chaos
- aggregation quality suffers when every hop “helps” differently
- observability gets harder
- the end result becomes harder to reason about

Transitloom should gather health and capability information from relays and paths, but the actual split decision remains endpoint-owned.

This is part of the soul of the v1 data-plane design.

---

## Zero-overhead means accepting statefulness

Transitloom’s v1 raw UDP path values zero in-band overhead.

That has a cost:
- more state
- more explicit association context
- more careful binding logic
- more careful relay legality

That is acceptable.

The soul of the project is not “minimize implementation effort at any cost.”

It is:
- accept the right complexity
- in the right place
- for the right practical outcome

Stateful forwarding is one of those accepted costs.

---

## Performance-first does not mean reckless

Transitloom is performance-first for the flagship use case.

But that does not mean:
- chase every micro-optimization early
- write unreadable code
- accept architecture damage for local speed
- introduce clever scheduling tricks before observability exists
- assume benchmarks matter more than real deployment behavior

Performance-first means:
- choose the right transport boundaries
- choose the right scope boundaries
- choose the right scheduler defaults
- avoid architecture that destroys aggregation quality
- keep room for later optimization without corrupting the model

That is disciplined performance thinking, not reckless tuning.

---

## Simplicity is contextual

Transitloom should prefer simpler solutions, but “simple” must be judged at the system level.

A locally simple hack can create globally ugly consequences.

Examples of fake simplicity:
- collapsing certificate validity and participation permission
- treating discovery as authorization
- letting relays invent arbitrary forwarding behavior
- making WireGuard-specific assumptions inside the core model
- pretending multi-hop flexibility is free

Real simplicity is:
- explicit roles
- explicit state
- explicit legality
- explicit bindings
- clear package boundaries
- clear control vs data separation

Agents should optimize for **system simplicity**, not merely fewer lines of code.

---

## Build vertical truth, not horizontal illusion

Transitloom should be built in vertical slices.

A thin, real, end-to-end path is more valuable than many broad partial implementations.

For example, this is good:
- coordinator starts
- node enrolls
- node receives admission token
- service registers
- association exists
- direct raw UDP traffic works
- WireGuard works over that path

This is less useful:
- lots of abstract route types
- many placeholder transport interfaces
- large policy framework
- no real working direct path

The soul of Transitloom prefers working vertical truth over horizontal illusion.

---

## The project should stay reviewable

Transitloom is ambitious enough already.

That means it must stay reviewable:
- reviewable in code
- reviewable in commits
- reviewable in architecture
- reviewable in task boundaries

Agents should avoid:
- giant mixed-purpose patches
- silent semantic changes
- terminology drift
- hidden architecture changes inside “refactor” commits

A project of this kind becomes fragile if it stops being reviewable.

---

## Memory is part of the system

Because coding agents are context-limited, project memory is not optional.

The `agents/` workspace is part of how Transitloom stays coherent over time.

Agents should treat:
- `agents/CONTEXT.md`
- `agents/MEMORY.md`
- `agents/TASKS.md`
- related files under `agents/`

as part of the system’s continuity layer.

Failing to update them when needed is not just a documentation lapse.  
It is a project-coherence failure.

---

## What should make an agent uneasy

An agent should become cautious when a change starts to look like:

- “this broadens v1 scope, but maybe nobody will notice”
- “this makes WireGuard special in the core, but it is convenient”
- “this collapses an important distinction, but the code is shorter”
- “this relay behavior is clever, but nobody can reason about it”
- “this avoids updating the specs, but the implementation is moving anyway”
- “this seems fine locally, but it shifts the product identity quietly”

Those are usually signals to slow down and reevaluate.

---

## What good work feels like in Transitloom

Good work in Transitloom tends to have these qualities:

- the architecture stays intact
- names become clearer, not muddier
- the next implementation step gets easier
- the flagship use case gets closer to working
- the code becomes more understandable
- durable knowledge gets written down
- complexity is moved into the right layer instead of merely hidden

That is the feeling agents should aim for.

---

## One-sentence soul

If you need one sentence to guide decisions while coding, use this:

**Transitloom should become a practical, disciplined transport fabric for real networks by getting the generic core and the flagship raw-UDP path right before chasing broader mesh ambition.**

---
