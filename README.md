# Transitloom

Transitloom is an open source overlay mesh transport platform for carrying UDP services across direct paths, relays, and multi-WAN networks.

It is designed for real-world environments where nodes may have multiple uplinks, dynamic IP addresses, NAT or CGNAT, private intranet reachability, and a need to combine direct connectivity, relay-assisted paths, and centralized coordination into one transport system.

Transitloom’s first flagship use case is **WireGuard over mesh**, but the project is designed around a more general service-carriage model rather than being limited to a single protocol.

## Why Transitloom

Modern networks are messy:

- nodes may sit behind NAT or CGNAT
- public addresses may change
- multiple WAN links may be available
- some paths may be cheap, some metered, some unstable
- some peers may be reachable directly, some only through relays
- private or intranet paths may exist and should be used when beneficial

Transitloom aims to make these environments easier to work with by providing a managed overlay transport layer that can discover, coordinate, and carry traffic across the available paths.

## Project goals

Transitloom is being built with these goals in mind:

- high-performance raw UDP transport over overlay paths
- practical multi-WAN bandwidth aggregation
- direct, intranet, and relay-assisted connectivity
- coordinator-managed trust, admission, and policy
- generic service carriage rather than a protocol-specific tunnel
- a strong fit for WireGuard-over-mesh deployments
- a clean foundation for future expansion

## Initial focus

The first major use case for Transitloom is:

- **WireGuard over mesh**
- **multi-WAN raw UDP aggregation**
- **real-world overlay connectivity across NAT, relays, and mixed paths**

The core platform, however, is intended to stay generic enough to support additional UDP-based services in the future.

## Design direction

Transitloom is currently being designed around a few key ideas:

- **coordinators** provide managed control, trust, discovery, policy, and relay assistance
- **nodes** expose services and carry traffic across the mesh
- **services** and **associations** are first-class concepts
- **raw UDP carriage** is the primary transport focus
- **zero in-band overhead** is an important goal for raw UDP data paths
- **WireGuard** should work over Transitloom without requiring WireGuard protocol changes

## Planned capabilities

Transitloom is intended to support combinations of:

- direct public connectivity
- private or intranet connectivity
- coordinator-assisted discovery and relay
- node-assisted relay
- multi-path and multi-WAN transport
- policy-controlled admission and revocation
- stable local service bindings for applications using the mesh

## Project status

Transitloom is in an early public stage.

The architecture is still being refined, and the project is not yet production-ready. Expect substantial design work, iteration, and breaking changes before a stable release exists.

## License

Transitloom is licensed under the **GNU General Public License v3.0**.

See [`LICENSE`](LICENSE) for details.

## Repository

GitHub: <https://github.com/zhouchenh/transitloom>

## Contributing

The project is still at an early stage, so design discussions, issue reports, and thoughtful feedback are especially valuable.

More contributor guidance and project documentation will be added as the architecture and implementation take shape.
