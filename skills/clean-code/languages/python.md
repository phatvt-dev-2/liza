# Python — Clean Code Language Profile

## Tool Map

| Variable | Command | Notes |
|----------|---------|-------|
| `$TEST_CMD` | `pytest -q` | |
| `$COVERAGE_CMD` | `pytest --cov --cov-report=xml` | Requires `pytest-cov` |
| `$COVERAGE_REPORT` | `coverage.xml` | Cobertura XML — compatible with `diff-cover` |
| `$TYPE_CHECKER` | `mypy <files>` | Or `pyright` if project uses it |
| `$DEAD_CODE_TOOL` | `vulture <files>` | High false-positive rate — require per-item approval |
| `$IMPORT_CHECKER` | `isort --check <files>` | Import sorting/cleanup. Or `autoflake` for unused imports |
| `$CYCLE_CHECKER` | `import-linter` or `pydeps` | Or manual: attempt import from a clean Python process |

If `pytest-cov` is unavailable: fall back to `coverage run -m pytest && coverage xml -o coverage.xml`. If `coverage` itself is unavailable: warn that coverage gating is disabled, require explicit waiver to proceed. If `mypy`/`pyright` unavailable: warn, require explicit waiver for type checking.

## Performance Patterns

| Transformation | Potential perf impact |
|---------------|---------------------|
| List comprehension → generator | Lower memory, but recomputes; not always a win |
| `dict` → `dataclass` | Attribute access faster with `__slots__`; construction slower than dict literal. Net depends on read/write ratio. |
| String concat → f-string | Generally faster — but watch `.format()` in tight loops |

## Idiom Patterns

| Signal | Transformation |
|--------|----------------|
| Raw `dict` for structured data | Use `dataclass` or `TypedDict` |
| `os.path` manipulation | Use `pathlib.Path` |
| Manual resource cleanup (`open`/`close`) | Use context managers (`with`) |
| List comprehension not materialized | Use generator expression |
| Missing `__slots__` on data-heavy classes | Add `__slots__` for memory efficiency |
| `isinstance` chains | Consider `match`/`case` (Python ≥3.10) or dispatch |
| Mutable default arguments (`def f(x=[])`) | Use `None` sentinel + assignment in body |
| Bare `except:` or `except Exception:` | Catch specific exceptions |
| String formatting with `%` or `.format()` | Use f-strings |
| Manual `__init__` + `__repr__` + `__eq__` boilerplate | Use `@dataclass` |
