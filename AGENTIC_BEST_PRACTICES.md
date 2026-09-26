# Agentic Programming Best Practices

**Date**: 2026-09-25  
**Reference**: Current best practices for Claude agents and multi-agent systems  
**Compiled from**: Anthropic docs, industry guides, and research (2026)

---

## Core Principles

### 1. Clear Task Specification
- Specify outcomes, constraints, and verification criteria explicitly
- Name the file, scenario, and testing approach clearly
- Define what "done" looks like before starting
- Specify language, framework versions, and performance requirements

### 2. Planning & Verification Over Assertion
- Have agents show evidence (test output, file diffs) rather than just claiming success
- Use `/clear` between major phases to prevent context drift
- Separate research/planning from implementation to avoid solving the wrong problem

### 3. Session & Prompt Management
- Keep prompts minimal and clear to avoid contradictory instructions
- Provide only needed tools and context (reduces noise)
- Use separate sessions for distinct problem domains

---

## Tool Design (Critical for Agent Success)

### 4. Tool Definition
- Provide **extremely detailed descriptions** for each tool (most important factor in tool performance)
- Use semantic, stable identifiers in tool responses (not opaque internal IDs)
- Design responses to return only high-signal information
- Use meaningful namespacing: `db_query`, `github_list_prs` (prefix with resource type)

### 5. Tool Safety
- Validate all inputs at handler boundaries
- Avoid string concatenation for SQL/shell commands
- Use parameterized queries, not string interpolation
- Allowlist tool access by resource
- Treat every user message as potentially hostile

---

## Multi-Agent System Patterns

### 6. Architecture Patterns
- **Sequential**: One agent per phase (research → plan → implement → test)
- **Hierarchical**: Orchestrator agent delegates to specialists (code-review agent, test agent, deploy agent)
- **Blackboard**: Shared state/context (memory system, shared files)
- **Market-based**: Agents bid for tasks based on capability

### 7. Autonomy & Control
- Most production systems carefully constrain agent autonomy
- Require human approval for risky actions (destructive operations, deploys)
- Build in feedback loops and review gates
- Use permissions: deny-by-default, tight allowlist

### 8. Resilience & Monitoring
- Persistent checkpointing to database (SQLite, PostgreSQL, Redis)
- Handle errors gracefully (don't fail silently)
- Log all agent decisions for audit trails
- Monitor for runaway loops and infinite retries

---

## Common Failure Modes to Avoid

- Ambiguous tool schemas
- Missing error handling on executor side
- Runaway loops (tool calls that don't converge)
- Tools the model thinks exist but don't
- Out-of-scope edits (wrong file, wrong scope)
- Weak permission boundaries

---

## Implementation Patterns

### 9. Skills & Reusability
- Use Skills for domain-specific expertise (progressive disclosure)
- Load capabilities on-demand so knowledge transfers across sessions
- Document what each skill/tool does (don't repeat guidance)

### 10. Verification & Self-Correction
- Store results to files for review
- Use assertions and tests to verify outputs
- Build in self-correction when outputs fail validation
- Show work (diffs, test results) rather than just assertions

---

## Key Resources & Links

### Official Anthropic Documentation
- [Claude Code Best Practices - Official](https://code.claude.com/docs/en/best-practices)
- [Build with Claude / Skills Guide](https://platform.claude.com/docs/en/build-with-claude/skills-guide)
- [Agent Skills Overview](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview)
- [Tool Use Tutorial](https://platform.claude.com/docs/en/agents-and-tools/tool-use/build-a-tool-using-agent)
- [Claude API Skill Guide](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/claude-api-skill)

### 2026 Guides & Best Practices
- [10 Claude Code Best Practices for Agentic Coding: A 2026 Guide](https://www.openhands.dev/blog/claude-code-best-practices-agentic-coding)
- [Agentic Coding 2026: AI Agent Teams Guide](https://halallens.no/en/blog/agentic-coding-in-2026-the-complete-guide-to-plugins-multi-model-orchestration-and-ai-agent-teams)
- [Agentic Coding Guide 2026: Claude Code, Codex & 5 Patterns](https://www.teamday.ai/blog/complete-guide-agentic-coding-2026)
- [Claude Code Best Practices for Agentic Coding](https://thoughtminds.ai/blog/claude-code-best-practices-for-agentic-coding-in-modern-software-development)
- [AI Coding Agents in 2026: Best Tools, Workflows, and Risks](https://aiidelist.com/blog/ai-coding-agent-2026)
- [Claude Custom Tool Best Practices (2026 Guide)](https://www.getclaudeskills.com/blog/claude-custom-tool-definition-best-practices)

### Multi-Agent System Architecture
- [Multi-Agent Systems: Design Patterns and Orchestration](https://tetrate.io/learn/ai/multi-agent-systems)
- [Building Multi-Agent AI Systems: Architecture Patterns and Best Practices](https://dev.to/matt_frank_usa/building-multi-agent-ai-systems-architecture-patterns-and-best-practices-5cf)
- [Databricks Agent System Design Patterns](https://docs.databricks.com/aws/en/generative-ai/guide/agent-system-design-patterns)
- [Multi-Agent System Patterns: Architectures, Roles & Design](https://medium.com/@mjgmario/multi-agent-system-patterns-a-unified-guide-to-designing-agentic-architectures-04bb31ab9c41)
- [Four Design Patterns for Event-Driven, Multi-Agent Systems](https://www.confluent.io/blog/event-driven-multi-agent-systems/)
- [Google's Eight Essential Multi-Agent Design Patterns](https://www.infoq.com/news/2026/01/multi-agent-design-patterns/)
- [A Practical Guide to the Architectures of Agentic Applications](https://www.speakeasy.com/mcp/using-mcp/ai-agents/architecture-patterns)

### Research Papers
- [LLM-Enabled Multi-Agent Systems: Empirical Evaluation and Insights into Emerging Design Patterns](https://arxiv.org/pdf/2601.03328)
- [Unified-MAS: Universally Generating Domain-Specific Nodes for Empowering Automatic Multi-Agent Systems](https://arxiv.org/pdf/2603.21475)

### Tool Use & Production Patterns
- [Claude Agents SDK: Best Practices From the Team](https://bertomill.medium.com/claude-agents-sdk-best-practices-from-the-team-that-built-it-63580d1a0c3b)
- [Tool Use in the Claude API: Production Patterns for Reliable Agents](https://www.developersdigest.tech/blog/tool-use-claude-api-production-patterns)

### How Claude Code is Used in Practice
- [Claude Code Expertise Research - Anthropic](https://www.anthropic.com/research/claude-code-expertise)
- [Engineering at Anthropic](https://www.anthropic.com/engineering/claude-code-best-practices)

---

## TL;DR

Focus on:
1. **Detailed tool definitions** — most important factor in agent success
2. **Clear task specs with verification** — define what "done" means
3. **Constrained autonomy with permissions** — deny-by-default, tight allowlist
4. **Resilience & checkpointing** — persistent state, graceful error handling
5. **Separating concerns across specialized agents** — sequential, hierarchical, or blackboard patterns

---

## Application to RSS.Chat Go Project

When handing tasks to agents from `SUBTASKS.md`:

1. **Be specific**: "Complete T1-2b from SUBTASKS.md" (not generic "implement WebSub")
2. **Show evidence**: Have agent verify with tests, file diffs, compiler output
3. **Constrain scope**: Each subtask is 1-3 hours, has dependencies listed
4. **Verify completion**: Check acceptance criteria, not just assertions
5. **Separate concerns**: Different agents for different tier/feature areas if needed

Reference this file in agent prompts to ensure consistent quality across contributors.
