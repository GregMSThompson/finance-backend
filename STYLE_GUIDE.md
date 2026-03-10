# Style Guide

## Function Ordering

Goal: make files easy to scan by showing the public API first and implementation details second.

Rules:

1. Place exported (public) functions and methods at the top of the file.
2. Place unexported (private) helpers below exported functions.
3. Keep helpers close to the public function they support when practical.
4. If the file is large, group by feature/flow first; within each group, keep public functions above private helpers.
5. Prefer readability over rigid ordering if strict ordering would make the file harder to follow.

Notes:

- Constructors like `NewX(...)` are exported API and should appear near the top.
- Interface implementations can stay together if splitting them harms readability.

## Validation Placement

Goal: keep transport concerns in handlers and business rules in services.

Rules:

1. Perform HTTP/transport validation in handlers.
2. Perform business/domain validation in services.
3. Do not duplicate the same business rule in both handler and service.
4. Keep defaults and cross-field validation in services.
5. Return typed/domain errors from services; map those to HTTP responses in handlers.

Handler validation includes:

- request body decoding/parsing errors
- missing required request fields
- basic shape/type checks tied to the API contract

Service validation includes:

- allowed value sets and business constraints
- cross-field rules
- defaults and normalization
- state/ownership checks and invariants

## Comments

Goal: comments should add context, not restate obvious code.

Rules:

1. Comment exported types, functions, and methods with Go-style doc comments.
2. Do not add comments that simply restate what the code already says.
3. Add comments for non-obvious intent, business rules, constraints, and tradeoffs.
4. Add short comments before complex blocks where the "why" is not clear from code alone.
5. Keep comments accurate and update/remove them when behavior changes.
6. Prefer clear names and small functions over excessive inline comments.

Good comment use cases:

- explaining why a query/index/order is required
- documenting invariants and edge-case handling
- clarifying temporary workarounds with context
- describing assumptions around external services or APIs

Avoid:

- line-by-line narration of simple assignments/branches
- redundant comments on self-explanatory code
- stale TODO comments without context
