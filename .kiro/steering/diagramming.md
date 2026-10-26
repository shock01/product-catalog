# Diagramming

## Standard
All diagrams are written in **Mermaid** and embedded directly in Markdown files using fenced code blocks:

````markdown
```mermaid
...
```
````

Never use external diagram tools (draw.io, Lucidchart, etc.) or binary image files for architecture diagrams. Mermaid source is the single source of truth — it lives in the repo and diffs cleanly in PRs.

## Where Diagrams Live
| File | Purpose |
|---|---|
| `DIAGRAM.md` | Top-level infrastructure and data-flow overview |
| `docs/adr/ADR-*.md` | Sequence or flow diagrams scoped to the decision being documented |
| `docs/domain/` | Domain model, bounded context, and event diagrams |

## Diagram Types to Use
| Scenario | Mermaid type |
|---|---|
| Infrastructure / component overview | `graph TD` or `graph LR` |
| Request / async flow | `sequenceDiagram` |
| Domain events and state transitions | `stateDiagram-v2` |
| Data model / entity relationships | `erDiagram` |
| Bounded context map | `graph LR` with subgraphs |
| CI/CD or process flow | `flowchart TD` |

## Update Rule
Whenever infrastructure, domain events, or API contracts change, the relevant Mermaid diagram(s) **must** be updated in the same PR. Stale diagrams are treated as bugs.
