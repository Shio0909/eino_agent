---
name: codebase-understanding
description: |
  Systematically understand a codebase using source evidence. Use when the user asks
  how code works, where behavior is implemented, how modules connect, what calls what,
  where a symbol is defined, how a feature flows through the system, or asks for
  code-grounded explanations before changing implementation.
---

# Codebase Understanding

Use this skill when the task is to understand existing code before answering or changing it. The goal is evidence-backed comprehension, not broad architecture documentation.

## Use This Skill For

| Need | Use This Skill? |
|------|-----------------|
| Locate where a behavior is implemented | Yes |
| Explain a function, handler, service, or feature flow | Yes |
| Trace definitions, callers, callees, or module relationships | Yes |
| Compare what code actually does against docs or assumptions | Yes |
| Generate a full architecture blueprint | No, use `architecture-blueprint-generator` |
| Design GraphRAG or code graph storage patterns | No, use `graphrag-system-design` or `graphrag-patterns` |
| Implement Eino agent/tool mechanics | No, use `eino-agent` or `eino-component` |

## Investigation Workflow

1. **Classify the question** — Identify whether the user needs location, behavior explanation, call chain, architecture slice, bug root cause, or change impact.
2. **Search before reading** — Use lightweight search first to find candidate symbols, routes, configs, tests, and docs.
3. **Read primary evidence** — Read the implementation files that own the behavior. Do not answer from filenames or search hits alone.
4. **Cross-check with adjacent code** — Check callers, tests, configuration, and route wiring when the answer depends on runtime flow.
5. **Use graph tools for structure** — For definitions, call chains, file structure, repo overview, or cross-module relationships, prefer code graph queries when available.
6. **Synthesize with evidence** — Answer from the code path you verified, and separate confirmed facts from reasonable inference.

## Tool Strategy

Use text search for fast narrowing:

- `grep` / content search: find symbols, routes, config keys, errors, tests, and feature names.
- `find` / file search: locate likely handlers, services, repositories, configs, migrations, and docs.
- `read`: inspect the actual implementation after narrowing candidates.

Use code graph tools for structural questions:

- `definition`: locate where a symbol is defined.
- `call_chain`: inspect callers or callees.
- `structure`: summarize entities in a file.
- `search`: find related symbols by name.
- `overview`: understand indexed repository scale and major graph contents.

Prefer this order unless the user asks specifically for graph analysis:

```text
search symbols -> read owning files -> inspect wiring/tests/config -> query graph for structure -> answer with evidence
```

## Evidence Rules

- Do not invent files, functions, routes, flags, or call relationships.
- Do not treat comments, docs, or names as truth when implementation is available.
- A cross-file conclusion needs cross-file evidence.
- A runtime-flow answer should check the entry point and the business logic it calls.
- A configuration answer should check defaults and where the config is consumed.
- If evidence is incomplete, say what is confirmed and what remains unverified.

## Stop Conditions

Stop searching and answer when one of these is true:

- The owning implementation and its entry/wiring point are verified.
- The relevant definition and caller/callee path are verified.
- Search results are repeating and no new evidence appears.
- The remaining uncertainty is not worth more tool calls; state the uncertainty directly.
- The tool-call budget is reached; answer from the strongest evidence found.

## Output Pattern

For concise answers:

```text
Conclusion: ...
Evidence:
- path:line — what this proves
- path:line — what this proves
Limit: ...
```

For deeper explanations:

1. Start with the direct answer.
2. Walk the code path in runtime order.
3. Cite key files with `path:line`.
4. Explain design implications or tradeoffs only after the evidence.
5. End with current limitations or next verification step when relevant.

## Quality Bar

A good answer produced with this skill:

- Is grounded in actual source files.
- Names the concrete code path, not only components.
- Uses line-referenced evidence for important claims.
- Avoids broad rewrites unless the user asked for implementation.
- Stops once enough evidence exists instead of endlessly exploring.
