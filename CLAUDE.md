# Claude Code Instructions

Before making any code changes, read and follow `STYLE_GUIDE.md`.

Requirements:

1. Apply the conventions in `STYLE_GUIDE.md` to all edits.
2. If there is a conflict between existing code and `STYLE_GUIDE.md`, prefer `STYLE_GUIDE.md` for new or modified code.
3. Keep changes consistent with nearby code unless doing so would violate `STYLE_GUIDE.md`.
4. Do not introduce new style patterns without updating `STYLE_GUIDE.md`.
5. Consult the user and get explicit confirmation before making any updates to `STYLE_GUIDE.md`.

## Go Checks

Use `go vet ./...` to verify code correctness. Do not use `go build` — it produces binary artifacts in the working directory.
