#!/usr/bin/env python3
"""
Convert Grafana dashboards from MySQL to PostgreSQL datasource.
Uses sqlglot for AST-based SQL parsing and transformation.
"""

import json
import sys
import re
from pathlib import Path
from typing import Any, Dict

try:
    import sqlglot
except ImportError:
    print("ERROR: sqlglot not installed. Run: pip install -r requirements.txt")
    sys.exit(1)


def convert_sql_mysql_to_postgres(sql: str) -> str:
    """
    Convert MySQL SQL to PostgreSQL using sqlglot AST parsing.
    Falls back to regex for patterns sqlglot doesn't handle.
    """
    if not sql or not sql.strip():
        return sql

    try:
        # Protect string literals from function conversions
        sql, string_literals = protect_string_literals(sql)

        # Protect Grafana template variables from sqlglot conversion
        # sqlglot treats ${var} as MySQL user variables → STRUCT(var)
        sql, placeholders = protect_grafana_variables(sql)

        # Try sqlglot transpilation first
        converted = sqlglot.transpile(sql, read='mysql', write='postgres')[0]

        # Post-process patterns sqlglot might miss
        converted = post_process_sql(converted)

        # Restore Grafana macros to lowercase (sqlglot uppercases them)
        converted = restore_grafana_macros(converted)

        # Restore Grafana template variables
        converted = restore_grafana_variables(converted, placeholders)

        # Restore string literals
        converted = restore_string_literals(converted, string_literals)

        return converted
    except Exception as e:
        # Fallback to regex-based conversion for complex cases
        print(f"  [WARN] sqlglot failed, using regex fallback: {str(e)[:100]}")
        return regex_fallback_conversion(sql)


def protect_string_literals(sql_text: str) -> tuple[str, dict]:
    """
    Replace all string literals with placeholders to protect them from conversion.
    Returns modified SQL and mapping of placeholders to original strings.
    """
    literals = {}
    counter = 0
    result = []
    i = 0

    while i < len(sql_text):
        if sql_text[i] in ("'", '"'):
            quote_char = sql_text[i]
            literal_parts = [sql_text[i]]
            i += 1
            while i < len(sql_text):
                literal_parts.append(sql_text[i])
                if sql_text[i] == quote_char:
                    # Check for escaped quote ('' or "")
                    if i + 1 < len(sql_text) and sql_text[i + 1] == quote_char:
                        literal_parts.append(sql_text[i + 1])
                        i += 2
                    else:
                        i += 1
                        break
                else:
                    i += 1

            literal = ''.join(literal_parts)
            placeholder = f'STRING_LITERAL_{counter}_PLACEHOLDER'
            literals[placeholder] = literal
            result.append(placeholder)
            counter += 1
        else:
            result.append(sql_text[i])
            i += 1

    return ''.join(result), literals


def restore_string_literals(sql_text: str, literals: dict) -> str:
    """Restore string literals from placeholders."""
    for placeholder, literal in literals.items():
        sql_text = sql_text.replace(placeholder, literal)
    return sql_text


def replace_function_with_balanced_parens(sql_text: str, func_pattern: str, replacement_func) -> str:
    """
    Generic function replacer that handles nested parentheses via depth counting.

    Args:
        sql_text: SQL string to process (string literals already protected)
        func_pattern: Regex pattern for function name (e.g., r'\\bHOUR\\s*\\(')
        replacement_func: Function taking matched argument and returning replacement string

    Returns:
        Modified SQL with function replaced
    """
    result = []
    i = 0
    pattern = re.compile(func_pattern, re.IGNORECASE)
    while i < len(sql_text):
        match = pattern.search(sql_text, i)
        if not match:
            result.append(sql_text[i:])
            break
        result.append(sql_text[i:match.start()])
        start = match.end()
        depth = 1
        j = start
        while j < len(sql_text) and depth > 0:
            if sql_text[j] == '(':
                depth += 1
            elif sql_text[j] == ')':
                depth -= 1
            j += 1
        if depth == 0:
            arg = sql_text[start:j-1]
            result.append(replacement_func(arg))
            i = j
        else:
            result.append(match.group(0))
            i = start
    return ''.join(result)


def post_process_sql(sql: str) -> str:
    """
    Post-process SQL after sqlglot conversion for Grafana-specific patterns.
    """
    # Remove backticks (PostgreSQL uses double quotes or no quotes)
    sql = sql.replace('`', '"')

    # CURDATE() -> CURRENT_DATE
    sql = re.sub(r'\bCURDATE\s*\(\s*\)', 'CURRENT_DATE', sql, flags=re.IGNORECASE)

    # FORMAT(value, decimals) -> ROUND(value, decimals)
    # MySQL FORMAT() formats number with commas and decimal places
    # PostgreSQL ROUND() works - auto-casts to text in CONCAT context
    # Don't add ::text here because FORMAT has nested functions with commas
    # and [^,]+ regex can't handle them properly
    sql = re.sub(
        r'\bFORMAT\s*\([^)]*\)',
        lambda m: m.group(0).replace('FORMAT', 'ROUND', 1),
        sql,
        flags=re.IGNORECASE
    )

    # Cast Grafana time macros to timestamp when used with INTERVAL
    # $__timeFrom() + INTERVAL → $__timeFrom()::timestamp + INTERVAL
    # $__timeTo() + INTERVAL → $__timeTo()::timestamp + INTERVAL
    # Also handle when wrapped in parentheses: ($__timeTo() - INTERVAL
    sql = re.sub(
        r'(\$__timeFrom\(\)|\$__timeTo\(\))\s*([+\-])\s*INTERVAL',
        r'\1::timestamp \2 INTERVAL',
        sql,
        flags=re.IGNORECASE
    )
    # Handle parentheses-wrapped time macros: ($__timeFrom() or ($__timeTo()
    sql = re.sub(
        r'\((\$__timeFrom\(\)|\$__timeTo\(\))\s*([+\-])\s*INTERVAL',
        r'(\1::timestamp \2 INTERVAL',
        sql,
        flags=re.IGNORECASE
    )

    # Remove CHARACTER SET from CAST (MySQL-specific)
    # CAST(col AS CHAR CHARACTER SET utf8mb4) → CAST(col AS TEXT)
    sql = re.sub(
        r'\bCAST\s*\(([^)]+)\s+AS\s+CHAR\s+CHARACTER\s+SET\s+\w+\)',
        r'CAST(\1 AS TEXT)',
        sql,
        flags=re.IGNORECASE
    )

    # CAST AS CHAR → CAST AS TEXT (PostgreSQL doesn't have bare CHAR for casting)
    # Use global replace to handle nested CAST expressions
    sql = re.sub(
        r'\s+AS\s+CHAR\)',
        ' AS TEXT)',
        sql,
        flags=re.IGNORECASE
    )

    # HOUR() -> EXTRACT(HOUR FROM ...)
    sql = replace_function_with_balanced_parens(
        sql,
        r'\bHOUR\s*\(',
        lambda arg: f'EXTRACT(HOUR FROM {arg})'
    )

    # Boolean comparisons: column = 1 → column = TRUE, column = 0 → column = FALSE
    # PostgreSQL schema has BOOLEAN type - does NOT accept integer comparison
    # ERROR: "operator does not exist: boolean = integer"
    # Must convert ALL boolean column comparisons: WHERE/AND/WHEN
    # Safe: CASE WHEN bool_col = TRUE THEN 1 - condition is boolean, result is integer
    # THEN/ELSE values determine CASE return type, not WHEN condition type
    sql = re.sub(
        r'(WHERE|AND|WHEN)\s+([\w.]*(?:has_|is_)[\w.]+|[\w.]+_flag)\s*=\s*1\b',
        r'\1 \2 = TRUE',
        sql,
        flags=re.IGNORECASE
    )
    sql = re.sub(
        r'(WHERE|AND|WHEN)\s+([\w.]*(?:has_|is_)[\w.]+|[\w.]+_flag)\s*=\s*0\b',
        r'\1 \2 = FALSE',
        sql,
        flags=re.IGNORECASE
    )

    # DAY_OF_WEEK -> EXTRACT(ISODOW FROM ...)
    # MySQL DAY_OF_WEEK() returns 1=Sunday, 2=Monday, 3=Tuesday...7=Saturday
    # PostgreSQL ISODOW returns 1=Monday, 2=Tuesday...7=Sunday
    sql = replace_function_with_balanced_parens(
        sql,
        r'\bDAY_OF_WEEK\s*\(',
        lambda arg: f'EXTRACT(ISODOW FROM {arg})'
    )

    # Fix weekday CASE mapping after DAY_OF_WEEK → ISODOW conversion
    # Old MySQL mapping: '2'→Monday, '3'→Tuesday...'1'→Sunday
    # New ISODOW mapping: '1'→Monday, '2'→Tuesday...'7'→Sunday
    weekday_replacements = {
        "WHEN '2' THEN '1.Monday'": "WHEN '1' THEN '1.Monday'",
        "WHEN '3' THEN '2.Tuesday'": "WHEN '2' THEN '2.Tuesday'",
        "WHEN '4' THEN '3.Wednesday'": "WHEN '3' THEN '3.Wednesday'",
        "WHEN '5' THEN '4.Thursday'": "WHEN '4' THEN '4.Thursday'",
        "WHEN '6' THEN '5.Friday'": "WHEN '5' THEN '5.Friday'",
        "WHEN '7' THEN '6.Saturday'": "WHEN '6' THEN '6.Saturday'",
        "WHEN '1' THEN '7.Sunday'": "WHEN '7' THEN '7.Sunday'"
    }
    for old, new in weekday_replacements.items():
        sql = sql.replace(old, new)

    # DOUBLE PRECISION -> NUMERIC
    # PostgreSQL ROUND() doesn't accept DOUBLE PRECISION, needs NUMERIC
    # Simple global replacement since NUMERIC is compatible everywhere DOUBLE PRECISION is used
    sql = re.sub(
        r'\bDOUBLE\s+PRECISION\b',
        'NUMERIC',
        sql,
        flags=re.IGNORECASE
    )

    # FIND_IN_SET -> ANY(string_to_array())
    # MySQL: FIND_IN_SET(value, csv_string) > 0
    # PostgreSQL: value = ANY(string_to_array(csv_string, ','))
    # Handle nested functions by matching balanced parentheses
    def replace_find_in_set(sql):
        pattern = r'FIND_IN_SET\s*\('
        result = []
        i = 0
        while i < len(sql):
            match = re.match(pattern, sql[i:], re.IGNORECASE)
            if match:
                start = i + match.end()
                # Find matching closing paren and split args
                depth = 1
                arg1_end = None
                for j in range(start, len(sql)):
                    if sql[j] == '(':
                        depth += 1
                    elif sql[j] == ')':
                        depth -= 1
                        if depth == 0:
                            # Found closing paren, now find the comma separator
                            for k in range(start, j):
                                if sql[k] == ',' and depth == 1:
                                    # Count depth properly
                                    temp_depth = 0
                                    valid_comma = True
                                    for m in range(start, k):
                                        if sql[m] == '(':
                                            temp_depth += 1
                                        elif sql[m] == ')':
                                            temp_depth -= 1
                                    if temp_depth == 0:
                                        arg1_end = k
                                        break
                            if arg1_end:
                                arg1 = sql[start:arg1_end].strip()
                                arg2 = sql[arg1_end+1:j].strip()
                                # Check if followed by > 0
                                rest_start = j + 1
                                gt_match = re.match(r'\s*>\s*0', sql[rest_start:])
                                if gt_match:
                                    result.append(f"{arg1} = ANY(string_to_array({arg2}, ','))")
                                    i = rest_start + gt_match.end()
                                    continue
                            break
                    elif sql[j] == ',' and depth == 1:
                        arg1_end = j
                result.append(sql[i])
                i += 1
            else:
                result.append(sql[i])
                i += 1
        return ''.join(result)

    # Simpler approach: iteratively replace innermost FIND_IN_SET first
    prev_sql = None
    while prev_sql != sql:
        prev_sql = sql
        sql = re.sub(
            r'FIND_IN_SET\s*\(([^(),]+),\s*([^()]+)\)\s*>\s*0',
            r"\1 = ANY(string_to_array(\2, ','))",
            sql,
            flags=re.IGNORECASE
        )

    # Fix malformed INTERVAL (missing number)
    # INTERVAL DAY -> INTERVAL '1 day'
    sql = re.sub(
        r'\bINTERVAL\s+DAY\b',
        "INTERVAL '1 day'",
        sql,
        flags=re.IGNORECASE
    )

    # ArgoCD images column is JSON - cast to text for GROUP BY/comparisons
    # Pattern: ri.images (without ::text already)
    # PostgreSQL JSONB requires explicit cast for equality/GROUP BY
    sql = re.sub(
        r'\bri\.images\b(?!\s*::)',
        'ri.images::text',
        sql,
        flags=re.IGNORECASE
    )

    # IFNULL -> COALESCE (if sqlglot missed it)
    sql = re.sub(r'\bIFNULL\s*\(', 'COALESCE(', sql, flags=re.IGNORECASE)

    # UNIX_TIMESTAMP -> EXTRACT(EPOCH FROM ...)
    sql = replace_function_with_balanced_parens(
        sql,
        r'\bUNIX_TIMESTAMP\s*\(',
        lambda arg: f'EXTRACT(EPOCH FROM {arg})'
    )

    # GROUP_CONCAT -> STRING_AGG
    sql = replace_function_with_balanced_parens(
        sql,
        r'\bGROUP_CONCAT\s*\(',
        lambda arg: f"STRING_AGG({arg}, ',')"
    )

    # TIMESTAMPDIFF - handle sqlglot format: TIMESTAMPDIFF(end, start, unit)
    # PostgreSQL needs: EXTRACT(EPOCH FROM (end - start)) / divisor
    # Use generic function to handle nested parens in arguments
    def replace_timestampdiff(args):
        # Parse comma-separated args manually (can't use split because of nested parens)
        parts = []
        depth = 0
        current = []
        for char in args:
            if char == '(':
                depth += 1
                current.append(char)
            elif char == ')':
                depth -= 1
                current.append(char)
            elif char == ',' and depth == 0:
                parts.append(''.join(current).strip())
                current = []
            else:
                current.append(char)
        if current:
            parts.append(''.join(current).strip())

        if len(parts) >= 3:
            end, start, unit = parts[0], parts[1], parts[2].upper()
            # Map unit to divisor
            divisors = {'DAY': '86400', 'HOUR': '3600', 'MINUTE': '60', 'SECOND': '1'}
            divisor = divisors.get(unit, '1')
            if divisor == '1':
                return f'EXTRACT(EPOCH FROM ({end} - {start}))'
            else:
                return f'(EXTRACT(EPOCH FROM ({end} - {start}))/{divisor})'
        return f'TIMESTAMPDIFF({args})'  # Fallback if parse fails

    sql = replace_function_with_balanced_parens(
        sql,
        r'\bTIMESTAMPDIFF\s*\(',
        replace_timestampdiff
    )

    # DIV(TO_CHAR(date, 'YYYY-MM'), N) -> calculate month-based division
    # Used for bucketing dates into N-month periods (e.g., half-years with N=6)
    sql = re.sub(
        r'DIV\s*\(\s*TO_CHAR\s*\(\s*([^,]+?)\s*,\s*["\']YYYY-MM["\']\s*\)\s*,\s*(\d+)\s*\)',
        r'((EXTRACT(YEAR FROM \1) * 12 + EXTRACT(MONTH FROM \1)) / \2)',
        sql,
        flags=re.IGNORECASE
    )

    return sql


def protect_grafana_variables(sql: str) -> tuple:
    """
    Replace Grafana template variables with safe placeholders before sqlglot.
    sqlglot treats ${var} as MySQL user variables and converts to STRUCT(var).
    Returns: (modified_sql, dict of placeholders)
    """
    placeholders = {}
    counter = 0

    # Match Grafana variables: ${variable} or ${variable:format}
    pattern = r'\$\{([^}]+)\}'

    def replacer(match):
        nonlocal counter
        placeholder = f'GRAFANA_PLACEHOLDER_{counter}_GRAFANA'
        placeholders[placeholder] = match.group(0)
        counter += 1
        return placeholder

    protected_sql = re.sub(pattern, replacer, sql)
    return protected_sql, placeholders


def restore_grafana_variables(sql: str, placeholders: dict) -> str:
    """
    Restore Grafana template variables from safe placeholders.
    Also convert IN (${var}) to = ANY(ARRAY[${var}]) to handle empty variables.
    PostgreSQL rejects IN () with no values, but = ANY(ARRAY[]) is valid.
    """
    for placeholder, original in placeholders.items():
        sql = sql.replace(placeholder, original)

    # Convert IN (${var}) to = ANY(ARRAY[${var}]::text[])
    # Empty: column = ANY(ARRAY[]::text[]) → FALSE (typed empty array)
    # Values: column = ANY(ARRAY['val1','val2']::text[]) → works correctly
    sql = re.sub(
        r'(\w+(?:\.\w+)?)\s+IN\s+\(\$\{([^}]+)\}\)',
        r'\1 = ANY(ARRAY[${\2}]::text[])',
        sql,
        flags=re.IGNORECASE
    )

    return sql


def restore_grafana_macros(sql: str) -> str:
    """
    Restore Grafana macros to lowercase after sqlglot uppercases them.
    Grafana expects: $__timeFilter(), $__timeFrom(), $__timeTo()
    """
    sql = re.sub(r'\$__TIMEFILTER', '$__timeFilter', sql)
    sql = re.sub(r'\$__TIMEFROM', '$__timeFrom', sql)
    sql = re.sub(r'\$__TIMETO', '$__timeTo', sql)

    # Fix WEEKDAY function conversion
    # MySQL: DATE_SUB(DATE(x), INTERVAL WEEKDAY(DATE(x)) DAY)
    # PostgreSQL: CAST(x AS DATE) - (EXTRACT(ISODOW FROM x) - 1) * INTERVAL '1 day'
    # Pattern: CAST(x AS DATE) - INTERVAL 'WEEKDAY DAY'
    sql = re.sub(
        r'CAST\s*\(([^)]+)\s+AS\s+DATE\)\s*-\s*INTERVAL\s+[\'"]WEEKDAY\s+DAY[\'"]',
        r"CAST(\1 AS DATE) - (EXTRACT(ISODOW FROM \1) - 1) * INTERVAL '1 day'",
        sql,
        flags=re.IGNORECASE
    )

    return sql


def regex_fallback_conversion(sql: str) -> str:
    """
    Regex-based fallback for SQL patterns sqlglot can't handle.
    Handles nested functions and computed INTERVAL expressions.
    """
    # CURDATE() -> CURRENT_DATE
    sql = re.sub(r'\bCURDATE\s*\(\s*\)', 'CURRENT_DATE', sql, flags=re.IGNORECASE)

    # DATE(x) -> x::date
    sql = re.sub(r'\bDATE\s*\(\s*([^)]+)\s*\)', r'(\1)::date', sql, flags=re.IGNORECASE)

    # DATE_FORMAT -> TO_CHAR (basic patterns)
    sql = re.sub(
        r"DATE_FORMAT\s*\(\s*([^,]+),\s*'%Y-%m-%d'\s*\)",
        r"TO_CHAR(\1, 'YYYY-MM-DD')",
        sql,
        flags=re.IGNORECASE
    )
    sql = re.sub(
        r"DATE_FORMAT\s*\(\s*([^,]+),\s*'%Y/%m'\s*\)",
        r"TO_CHAR(\1, 'YYYY/MM')",
        sql,
        flags=re.IGNORECASE
    )
    sql = re.sub(
        r"DATE_FORMAT\s*\(\s*([^,]+),\s*'%Y-%m'\s*\)",
        r"TO_CHAR(\1, 'YYYY-MM')",
        sql,
        flags=re.IGNORECASE
    )

    # STR_TO_DATE -> TO_DATE
    sql = re.sub(r'\bSTR_TO_DATE\s*\(', 'TO_DATE(', sql, flags=re.IGNORECASE)

    # CONVERT(expr, type) -> CAST(expr AS type)
    sql = re.sub(
        r'CONVERT\s*\(\s*([^,]+),\s*([^)]+)\)',
        r'CAST(\1 AS \2)',
        sql,
        flags=re.IGNORECASE
    )

    # IF(cond, a, b) -> CASE WHEN cond THEN a ELSE b END
    sql = re.sub(
        r'\bIF\s*\(\s*([^,]+),\s*([^,]+),\s*([^)]+)\)',
        r'CASE WHEN \1 THEN \2 ELSE \3 END',
        sql,
        flags=re.IGNORECASE
    )

    # IFNULL -> COALESCE
    sql = re.sub(r'\bIFNULL\s*\(', 'COALESCE(', sql, flags=re.IGNORECASE)

    # DATE_ADD/DATE_SUB with INTERVAL
    # Handle: DATE_ADD(date, INTERVAL n DAY) -> date + INTERVAL 'n day'
    sql = re.sub(
        r'DATE_ADD\s*\(\s*([^,]+),\s*INTERVAL\s+([^)]+)\)',
        r'\1 + INTERVAL ''\2''',
        sql,
        flags=re.IGNORECASE
    )
    sql = re.sub(
        r'DATE_SUB\s*\(\s*([^,]+),\s*INTERVAL\s+([^)]+)\)',
        r'\1 - INTERVAL ''\2''',
        sql,
        flags=re.IGNORECASE
    )

    # Normalize INTERVAL syntax for PostgreSQL
    sql = re.sub(r"INTERVAL\s+'(\d+)\s+DAY'", r"INTERVAL '\1 days'", sql, flags=re.IGNORECASE)
    sql = re.sub(r"INTERVAL\s+'(\d+)\s+HOUR'", r"INTERVAL '\1 hours'", sql, flags=re.IGNORECASE)

    return sql


def process_dashboard_recursive(obj: Any, path: str = "") -> Any:
    """
    Recursively process dashboard JSON, converting SQL and datasource references.
    """
    if isinstance(obj, dict):
        result = {}
        for key, value in obj.items():
            current_path = f"{path}.{key}" if path else key

            # Convert rawSql fields
            if key == "rawSql" and isinstance(value, str):
                result[key] = convert_sql_mysql_to_postgres(value)
            # Convert datasource string references
            elif key == "datasource" and isinstance(value, str) and value == "mysql":
                result[key] = "postgresql"
            # Convert datasource object references
            elif key == "datasource" and isinstance(value, dict) and value.get("type") == "mysql":
                result[key] = {**value, "type": "postgres"}
            # Recursively process nested objects
            else:
                result[key] = process_dashboard_recursive(value, current_path)

        return result

    elif isinstance(obj, list):
        return [process_dashboard_recursive(item, f"{path}[{i}]") for i, item in enumerate(obj)]

    else:
        return obj


def convert_dashboard(input_path: Path, output_path: Path) -> None:
    """
    Convert a single MySQL dashboard to PostgreSQL.
    """
    try:
        # Read input dashboard
        with open(input_path, 'r', encoding='utf-8') as f:
            dashboard = json.load(f)

        # Process dashboard recursively
        converted = process_dashboard_recursive(dashboard)

        # Append -pg suffix to UID to avoid collisions
        if "uid" in converted and converted["uid"]:
            if not converted["uid"].endswith("-pg"):
                converted["uid"] = f"{converted['uid']}-pg"

        # Update title to indicate PostgreSQL variant (optional)
        if "title" in converted:
            if not "(PostgreSQL)" in converted["title"]:
                converted["title"] = f"{converted['title']} (PostgreSQL)"

        # Write output dashboard
        output_path.parent.mkdir(parents=True, exist_ok=True)
        with open(output_path, 'w', encoding='utf-8') as f:
            json.dump(converted, f, indent=2)

        print(f"✓ Converted: {input_path.name}")

    except json.JSONDecodeError as e:
        print(f"✗ ERROR: Invalid JSON in {input_path}: {e}")
        sys.exit(1)
    except Exception as e:
        print(f"✗ ERROR converting {input_path}: {e}")
        sys.exit(1)


def main():
    if len(sys.argv) < 3:
        print("Usage: python convert-mysql-to-postgresql.py <input_path> <output_path>")
        print("  input_path: MySQL dashboard JSON file or directory")
        print("  output_path: PostgreSQL dashboard JSON file or directory")
        sys.exit(1)

    input_path = Path(sys.argv[1])
    output_path = Path(sys.argv[2])

    if not input_path.exists():
        print(f"ERROR: Input path does not exist: {input_path}")
        sys.exit(1)

    # Single file conversion
    if input_path.is_file():
        convert_dashboard(input_path, output_path)

    # Directory conversion
    elif input_path.is_dir():
        json_files = list(input_path.glob("*.json"))
        if not json_files:
            print(f"WARNING: No JSON files found in {input_path}")
            return

        print(f"Converting {len(json_files)} dashboards from {input_path} to {output_path}")

        for json_file in json_files:
            output_file = output_path / json_file.name
            convert_dashboard(json_file, output_file)

        print(f"\n✓ Conversion complete: {len(json_files)} dashboards")

    else:
        print(f"ERROR: Input path is neither file nor directory: {input_path}")
        sys.exit(1)


if __name__ == "__main__":
    main()
