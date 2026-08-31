# Style Guide

## File Ordering

Goal: make files easy to scan by keeping declarations in a predictable order.

Rules:

1. Order top-level declarations as: constants, interfaces, structs, exported (public) functions and methods, then unexported (private) functions and methods.
2. Keep declarations of the same kind together where practical.
3. Always group all constant and type declarations (constants, interfaces, structs) at the top of the file, ahead of any functions or methods — even in large files. Types and constants must never be sprinkled between functions, so the full set of names a file defines is visible in one place.
4. Keep private helpers close to the public function they support when practical.
5. If the file is large, group the functions and methods below the type block by feature/flow; within each group, keep exported before unexported.
6. Prefer readability over rigid ordering of functions and methods if strict ordering would make the file harder to follow. This does not relax rule 3 — the type block stays at the top regardless.

Notes:

- Constructors like `NewX(...)` are exported API and should appear before private functions.
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

## Request DTOs

Goal: keep request handling consistent across handlers and services by using explicit DTO request types.

Rules:

1. HTTP request bodies should be decoded into DTO request structs.
2. Handlers should pass DTO request objects into services rather than unpacking them into multiple primitive arguments.
3. Service method signatures should prefer request DTOs for operation inputs when the data originates from an API request.
4. Request DTOs should live in the `dto` package, not in handlers or services.
5. Handlers may still pass path parameters or authenticated user context separately when that better reflects the boundary.

Use this pattern:

- handler decodes request body into `dto.SomeRequest`
- handler passes `dto.SomeRequest` to service
- service applies business validation and defaults to the DTO or data derived from it

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
