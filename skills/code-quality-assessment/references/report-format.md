# Code Quality Assessment Report

Date: YYYY-MM-DD (commit <short-sha>)
Repository: <name>
Mode: <Full Assessment | Reassessment (after <previous date>) | Targeted (<components>) | Enrichment (pass N[, <lens> lens]) | Quick Health Check>

## Repository Metrics Dashboard

- **Production Code**: X,XXX lines of [Language] across N files
- **Test Code**: X,XXX lines across N test files (N.NN:1 test-to-production ratio)
- **Test Functions**: N test cases
- **Documentation**: X,XXX lines across N files
- **Specifications**: X,XXX lines across N files
- **Dependencies**: N direct (minimal / moderate / heavy)
- **CI/CD**: [pipeline description, coverage reporting: yes/no, coverage enforcement: yes/no]
- **Pre-commit**: [N hooks / none configured]

## Executive Summary

[One paragraph: what the project is, its overall engineering quality, and the key trade-offs it makes.]

**Key Strengths:**
- [Strength with evidence]
- ...

**Areas for Improvement:**
- [Area with evidence]
- ...

**Overall Rating: [Grade] ([Short justification — state the deduction from the next-higher grade])**

---

## Detailed Subsystem Analysis

### [Subsystem Name] (`path/`) ★★★★☆

**Strengths:**
- [Specific, evidenced strength]

**Concerns:**
- [Specific, evidenced concern — file names, LOC counts, concrete observations]

### [Next Subsystem] ...

---

<!-- Cross-cutting sections: include ONLY those with substantive findings. Omit entirely if nothing meaningful to report. -->

## Testing & Quality Infrastructure ★★★★☆

**Strengths:**
- ...

**Concerns:**
- ...

---

## Pre-Commit & CI Pipeline ★★★★☆

**Strengths:**
- ...

**Concerns:**
- ...

---

## Documentation & Specifications ★★★★☆

**Strengths:**
- ...

**Concerns:**
- ...

---

## Refactoring Recommendations by Priority

### Priority 1: High Impact / Low Risk

#### 1.1 [Title]
- **What**: [Specific files/components to change and how]
- **Risk**: Low — [rationale]
- **Impact**: [What improves and why it matters]

### Priority 2: Medium Impact / Medium Risk

#### 2.1 [Title]
- **What**: ...
- **Risk**: Medium — [rationale]
- **Impact**: ...

### Priority 3: Strategic / Long-term

#### 3.1 [Title]
- **What**: ...
- **Risk**: [Low / Medium / High] — [rationale]
- **Impact**: ...

---

## Summary

[One paragraph overall assessment — quality relative to scope/constraints, strengths to preserve, primary risks, whether current state is appropriate or needs intervention.]

**Overall Rating: [Grade] ([Same justification as Executive Summary])**
