# WTF CLI Project Governance

This directory contains the constitutional framework and templates for the WTF CLI project.

## Structure

```
.specify/
├── memory/
│   └── constitution.md          # Project constitution (v1.0.0)
└── templates/
    ├── plan-template.md         # Implementation planning template
    ├── spec-template.md         # Feature specification template
    ├── tasks-template.md        # Task generation template
    └── commands/                # Workflow command definitions
```

## Constitution Overview

The **WTF CLI Constitution v1.0.0** establishes four core principles:

### I. Code Quality First
- Go conventions (`go fmt`, `go vet`, linting)
- Single-purpose functions (max 50 lines)
- Explicit error handling
- Comprehensive documentation

### II. Test-First Development (NON-NEGOTIABLE)
- TDD cycle: Red → Green → Refactor
- >80% unit test coverage target
- Integration and E2E tests required
- Mock external dependencies

### III. User Experience Consistency
- Predictable CLI interface
- Actionable error messages
- Consistent output formatting
- Sensible defaults

### IV. Performance Requirements
- <200ms time-to-first-output (excluding API)
- <10ms shell integration overhead
- <20MB binary size
- <50MB memory usage

## Using the Templates

### Feature Development Workflow

1. **Specify** (`/specify` command)
   - Use `spec-template.md` to define feature requirements
   - Focus on WHAT and WHY, not HOW
   - Mark ambiguities with `[NEEDS CLARIFICATION]`

2. **Plan** (`/plan` command)
   - Use `plan-template.md` to design implementation
   - Pass Constitution Check gates
   - Generate research, contracts, and data models

3. **Generate Tasks** (`/tasks` command)
   - Use `tasks-template.md` to create ordered task list
   - Follow TDD: tests before implementation
   - Mark parallel tasks with `[P]`

4. **Implement** (`/implement` command)
   - Execute tasks in dependency order
   - Verify Constitution compliance
   - Run quality gates before commit

## Constitution Compliance

All features MUST:
- Pass Constitution Check before and after design phase
- Document any principle violations with justification
- Maintain test coverage and code quality standards
- Follow performance and UX requirements

## Amendment Process

To amend the constitution:
1. Propose change in GitHub issue with rationale
2. Discuss impact on existing code
3. Require maintainer consensus
4. Update version (MAJOR.MINOR.PATCH)
5. Update dependent templates
6. Document in Sync Impact Report

## Version History

- **v1.0.0** (2025-10-02): Initial constitution ratified
  - Established four core principles
  - Defined testing standards
  - Set performance requirements
  - Created quality gates

---

For questions or amendments, see `.specify/memory/constitution.md`
