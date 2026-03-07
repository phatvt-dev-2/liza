# Agent Tools

Sub-contract for tool usage. Applies to all modes (Pairing, Liza, Subagent).
When a preferred tool is unavailable in the current session, fall through to the next option in the preference chain.

## Forbidden tools

Refer to Security Protocol

## Other authorized tools

Any non destructive tool by default.

## MCP Server & Plugin Integration

**Default Rule:** Use MCP tools first. Standard tools are fallback when MCP unavailable or inappropriate.

**Pre-Action Check:** Before file/search/web operations, identify the MCP tool that applies. If using standard tools instead, state why.

### Tool Defaults by Operation

| Operation | Default Tool | Fallback | Use Fallback When |
|-----------|--------------|----------|-------------------|
| Read multiple files | `mcp__filesystem__read_multiple_files` | Read | Single file only |
| Directory exploration | `mcp__jetbrains__list_directory_tree` | Glob | JetBrains unavailable |
| Code search | `mcp__jetbrains__search_in_files_by_text` | Grep | Regex needed, or <3 files |
| Symbol lookup | `mcp__jetbrains__get_symbol_info` | LSP | JetBrains unavailable |
| File edit | `mcp__morph-mcp__edit_file` | Edit | File >2000 lines (tool reliability limit) |
| Web content | WebFetch | `mcp__fetch__fetch` | Need raw HTML, pagination, or blocked |
| Current info / library discovery | `mcp__perplexity__perplexity_ask` | WebSearch | — |
| Library API docs | `mcp__context7__query-docs` | Ref | Unknown/niche library, need tutorials |
| Library tutorials/guides | `mcp__Ref__ref_search_documentation` | WebFetch | Ref returns nothing useful |
| Repo architecture | `mcp__deepwiki__ask_question` | WebFetch | — |
| Code quality check | `mcp__jetbrains__get_file_problems` | — | After edits |

### Codebase Exploration

| Question Type | Primary Tool | Fallback |
|-------------------------------------------|--------------|----------|
| Exact keyword ("TODO") | Grep | — |
| Find files by name | JetBrains `find_files_by_name_keyword` | Glob |
| Semantic code search ("how does X work?") | **Warp Grep** (`mcp__morph-mcp__warpgrep_codebase_search`) | Task(Explore) only if WarpGrep insufficient |
| Symbol info at position | JetBrains `get_symbol_info` | LSP `hover` |
| Find references | LSP `findReferences` | Grep |
| Call hierarchy (callers/callees) | LSP `incomingCalls`/`outgoingCalls` | Task(Explore) if LSP not configured |
| Cross-file definitions | LSP `goToDefinition` | Task(Explore) if LSP not configured |
| Multi-file structural analysis | Task(Explore) | — |

**Tool characteristics:**
- **Grep**: Fastest, exact matches only, no synthesis
- **Warp Grep**: **first choice** for codebase understanding: targeted semantic discovery ("how does X work?"), especially when file paths are unknown. Fallback to native tools (Task Explore, rg) only if insufficient.
- **JetBrains** (when IDE available): Indexed, fast, includes docstrings and IDE diagnostics. Prefer over LSP for symbol info and workspace search
- **LSP**: Precise type info, references, call hierarchy (requires language server configured)
- **Task(Explore)**: Use for broad synthesis across many files, architectural relationship mapping, or when Warp Grep returns incomplete/ambiguous results.

**LSP prerequisite:** Language must have LSP configured (Python: `[tool.pyright]` in pyproject.toml; TS: tsconfig.json). If not configured, use Task(Explore) for call hierarchy and definitions.

### Tool Details

**JetBrains MCP**: IDE-aware operations via IntelliJ indexes. Use for indexed search on large codebases, symbol info, refactoring. Note: `get_file_problems` includes SonarQube issues.

**filesystem MCP**: Bulk/batch file operations — multi-file reads, recursive directory trees, file metadata.

**Morph-MCP**:
- *Fast Apply (`edit_file`)*: Shows only changed lines using `// ... existing code ...` placeholders. Avoids reading full files into context. Skip for files >2000 lines.
- *Warp Grep*: Multi-turn search subagent running parallel grep/read cycles. See "Codebase Exploration" section for when to use.

**fetch MCP**: Exact content without summarization — use when you need raw HTML, pagination, or WebFetch is blocked.

**perplexity**: Real-time web search with synthesis. Use for current info, library discovery, unfamiliar tech, external dependency issues.

**context7**: Structured API docs with code examples for well-known libraries. Two-step flow: `resolve-library-id` → `query-docs`. Best for "what's the API for X?" questions. High snippet density, consistent format.

**Ref**: Broader documentation search across web/GitHub. Better for tutorials, guides, niche libraries, or "how do I do X?" questions. Use `ref_read_url` to fetch specific doc pages found via search.

**deepwiki**: GitHub repo architecture and code structure.

**sequential-thinking-tools**: Structured reasoning with branching, backtracking, and revision tracking. Also provides tool recommendations for multi-step workflows. Use when competing hypotheses need parallel exploration, reasoning may need revision, or problem structure unclear. Skip for linear/obvious problems.

**postgres**: Read-only SQL — schema exploration, data validation, query-based analysis.

### Precedence

- **Local First**: IDE-integrated > local MCP > remote MCP > standard tools
- File operations: morph-mcp > jetbrains > filesystem > standard
- Web: WebFetch > fetch MCP (use fetch for exact content or pagination)
- Docs: context7 (API reference) > Ref (tutorials/niche) > deepwiki (repo architecture) > WebFetch (specific URL)

### Batching

Batch related operations within same MCP server when possible.
