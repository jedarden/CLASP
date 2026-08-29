# Changelog

All notable changes to CLASP will be documented in this file.

## [0.64.0] - 2026-08-29

### Added
- **MCP Server Translation Tools**: Implemented `clasp_translate_response` with real translation capabilities
- **LiteLLM Backend Provider**: Full support for LiteLLM as a routing backend with end-to-end integration tests
- **Stream Timeout for Reasoning Models**: Added specialized timeout handling for codex/reasoning models with extended UX
- **Prompt Cache Simulation**: Implemented prompt cache hit simulation in proxy handler with integration test coverage
- **Multi-Window Context Management**: Complete end-to-end context management across multiple windows
- **Hot-Reload Development Mode**: Added `.air.toml` configuration for rapid development iteration
- **Comprehensive Test Coverage**: Extensive new tests for provider tools, context scaling, LiteLLM routing, and non-OpenAI providers

### Changed
- **Kimi K2.5 & DeepSeek-R1 Support**: Translator now handles `reasoning_content` field from these models
- **Azure Error Handling**: Improved graceful error handling for Azure Responses API models
- **Documentation**: Updated README with current release info, Docker dynamic badge, and jedarden.com footer

### Fixed
- **Bead Workspace Migration**: Rehydrated workspace from bead-forge to bead-rs and cleaned up stale claims

### Technical
- Consolidated release pipeline via Argo workflow template
- Added ADR-1 documentation for reviving CLASP release pipeline
- Improved repository presentability with CI badge beads

---

## [0.63.0] - 2026-03-20

Previous release baseline
