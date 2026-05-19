#!/usr/bin/env python3
"""
Convert Grafana dashboards from MySQL to PostgreSQL datasource.
Uses sqlglot for AST-based SQL parsing and transformation.
"""

import json
import sys
import re
from pathlib import Path
from typing import Any

try:
    import sqlglot
except ImportError:
    print("ERROR: sqlglot not installed. Run: pip install -r requirements.txt")
    sys.exit(1)


def extract_balanced_parens(text: str, start_pos: int) -> tuple[str, int] | None:
    """
    Extract content from start_pos to matching closing paren with balanced counting.
    Returns (extracted_content, closing_paren_position) or None if no match.
    """
    paren_depth = 0
    i = start_pos

    while i < len(text):
        if text[i] == '(':
            paren_depth += 1
        elif text[i] == ')':
            if paren_depth == 0:
                return (text[start_pos:i], i)
            paren_depth -= 1
        i += 1

    return None


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

        # Fix $__timeFilter(MAX/MIN(...)) macro expansion issue (Step 5)
        # Must run AFTER restore_grafana_macros() to match lowercase $__timeFilter
        converted = fix_timefilter_aggregate_macro(converted)

        # Restore Grafana template variables
        converted = restore_grafana_variables(converted, placeholders)

        # Convert $interval(expr) pattern - Grafana variable used as function name
        # When $interval = 'DAYOFMONTH', becomes DAYOFMONTH(expr)
        # When $interval = 'WEEKDAY', becomes WEEKDAY(expr)
        # Replace with CASE statement for PostgreSQL
        pattern = re.compile(r'\$interval\s*\(', re.IGNORECASE)
        result_interval = []
        pos_interval = 0

        for match in pattern.finditer(converted):
            result_interval.append(converted[pos_interval:match.start()])
            extracted = extract_balanced_parens(converted, match.end())
            if extracted:
                expr, closing_pos = extracted
                replacement = f"CASE WHEN '$interval' = 'DAYOFMONTH' THEN EXTRACT(DAY FROM {expr}) ELSE (EXTRACT(ISODOW FROM {expr}) - 1) END"
                result_interval.append(replacement)
                pos_interval = closing_pos + 1
            else:
                result_interval.append(match.group(0))
                pos_interval = match.end()

        result_interval.append(converted[pos_interval:])
        converted = ''.join(result_interval)

        # Restore string literals
        converted = restore_string_literals(converted, string_literals)

        # Convert MySQL date format patterns to PostgreSQL (Step 1 - after string restoration)
        converted = convert_mysql_date_formats(converted)

        # Convert INTERVAL 'n DAY' * WEEKDAY(expr) -> (EXTRACT(ISODOW FROM expr) - 1) * INTERVAL 'n day'
        # Also handle negative case: INTERVAL 'n DAY' * -WEEKDAY(expr) -> -(EXTRACT(ISODOW FROM expr) - 1) * INTERVAL 'n day'
        # This pattern appears after sqlglot converts DATE_SUB(x, INTERVAL WEEKDAY(y) DAY)
        # Manual string building to handle nested parentheses properly
        pattern = re.compile(r"INTERVAL\s+'(\d+)\s+DAY'\s*\*\s*(-?)\s*WEEKDAY\s*\(", re.IGNORECASE)
        result = []
        pos = 0

        for match in pattern.finditer(converted):
            interval_num = match.group(1)
            negative_sign = match.group(2)
            result.append(converted[pos:match.start()])

            extracted = extract_balanced_parens(converted, match.end())
            if extracted:
                expr, closing_pos = extracted
                if negative_sign:
                    replacement = f"{negative_sign}(EXTRACT(ISODOW FROM {expr}) - 1) * INTERVAL '{interval_num} day'"
                else:
                    replacement = f"(EXTRACT(ISODOW FROM {expr}) - 1) * INTERVAL '{interval_num} day'"
                result.append(replacement)
                pos = closing_pos + 1
            else:
                result.append(match.group(0))
                pos = match.end()

        # Add remaining text after last match
        result.append(converted[pos:])
        converted = ''.join(result)

        # Convert standalone WEEKDAY(expr) calls (not multiplication pattern)
        # WEEKDAY(date) returns 0-6 (Mon-Sun) in MySQL, ISODOW returns 1-7 (Mon-Sun) in PostgreSQL
        # Pattern: WEEKDAY(...) BETWEEN/=/</>/>=/<=
        # Replace with: (EXTRACT(ISODOW FROM ...) - 1) to maintain 0-6 range
        pattern2 = re.compile(r"\bWEEKDAY\s*\(", re.IGNORECASE)
        result2 = []
        pos2 = 0

        for match in pattern2.finditer(converted):
            result2.append(converted[pos2:match.start()])

            extracted = extract_balanced_parens(converted, match.end())
            if extracted:
                expr, closing_pos = extracted
                replacement = f"(EXTRACT(ISODOW FROM {expr}) - 1)"
                result2.append(replacement)
                pos2 = closing_pos + 1
            else:
                result2.append(match.group(0))
                pos2 = match.end()

        # Add remaining text after last match
        result2.append(converted[pos2:])
        converted = ''.join(result2)

        # DIV(TO_CHAR(date, 'YYYY-MM'), N) -> calculate month-based division (after format conversion)
        # Used for bucketing dates into N-month periods (e.g., half-years with N=6)
        converted = re.sub(
            r'DIV\s*\(\s*TO_CHAR\s*\(\s*([^,]+?)\s*,\s*["\']YYYY-MM["\']\s*\)\s*,\s*(\d+)\s*\)',
            r'((EXTRACT(YEAR FROM \1) * 12 + EXTRACT(MONTH FROM \1)) / \2)',
            converted,
            flags=re.IGNORECASE
        )

        # Convert AS 'alias' to AS "alias" after string restoration
        # PostgreSQL doesn't accept single quotes for column aliases
        # Skip SQL type keywords (DATE, TIMESTAMP, etc.) - they should remain unquoted
        def convert_single_quoted_alias(match):
            alias = match.group(1)
            alias_upper = alias.upper()
            sql_type_keywords = {'DATE', 'TIME', 'TIMESTAMP', 'INT', 'INTEGER', 'BIGINT', 'SMALLINT',
                                'DECIMAL', 'NUMERIC', 'TEXT', 'VARCHAR', 'CHAR', 'BOOLEAN', 'BOOL',
                                'REAL', 'DOUBLE', 'FLOAT'}
            # Only treat as type keyword if exact uppercase match (e.g., 'TIMESTAMP' not 'Timestamp')
            if alias_upper in sql_type_keywords and alias == alias_upper:
                return f'AS {alias}'  # Unquote type keyword
            return f'AS "{alias}"'  # Quote regular alias

        converted = re.sub(r'\bAS\s+\'([^\']+)\'', convert_single_quoted_alias, converted, flags=re.IGNORECASE)

        # Quote bare (unquoted) column aliases to ensure case-sensitive matching
        # sqlglot inconsistently quotes aliases: subquery has "AS Date", outer query has "\"Date\""
        # PostgreSQL folds unquoted identifiers to lowercase, causing column not found errors
        # Pattern: AS <Word_starting_with_capital> (not already quoted, not a SQL type)
        # Exclude SQL types: DATE, TIME, TIMESTAMP, INT, INTEGER, BIGINT, TEXT, VARCHAR, etc.
        sql_types = r'(?!(?:DATE|TIME|TIMESTAMP|INT|INTEGER|BIGINT|SMALLINT|DECIMAL|NUMERIC|TEXT|VARCHAR|CHAR|BOOLEAN|BOOL|REAL|DOUBLE|FLOAT)\b)'
        converted = re.sub(
            rf'(?<!\")\bAS\s+{sql_types}([A-Z]\w*)\b',
            r'AS "\1"',
            converted
        )

        # Convert LIKE/NOT LIKE with double-quoted patterns to single quotes
        # MySQL accepts both, PostgreSQL only accepts single quotes for string literals
        # Double quotes in PostgreSQL are for identifiers, not strings
        converted = re.sub(r'\bLIKE\s+"([^"]+)"', r"LIKE '\1'", converted, flags=re.IGNORECASE)
        converted = re.sub(r'\bNOT\s+LIKE\s+"([^"]+)"', r"NOT LIKE '\1'", converted, flags=re.IGNORECASE)

        # Cast non-text columns to ::text for LIKE patterns
        # PostgreSQL LIKE requires text type, MySQL allows implicit conversion
        # Pattern: id LIKE '%value%' → id::text LIKE '%value%'
        #          id NOT LIKE '%x%' → id::text NOT LIKE '%x%'
        # Single-pass pattern matches both LIKE and NOT LIKE
        def add_text_cast_for_like_pattern(match):
            column = match.group(1)
            operator = match.group(2)  # "LIKE" or "NOT LIKE"
            # If column already has :: cast, don't add another
            if '::' in column:
                return match.group(0)
            return f'{column}::text {operator} '

        converted = re.sub(
            r'\b([\w.]+(?:::\w+)?)\s+((?:NOT\s+)?LIKE)\s+',
            add_text_cast_for_like_pattern,
            converted,
            flags=re.IGNORECASE
        )

        # Convert double-quoted string literals to single quotes
        # Pattern: = "value", <> "value", != "value", IN ("val1", "val2"), CONCAT(..., "text", ...), THEN "value"
        # But skip: column aliases after AS (already handled above as AS "alias")
        # MySQL accepts both quotes for strings, PostgreSQL only single quotes
        converted = re.sub(r'(=|<>|!=)\s+"([^"]+)"', r"\1 '\2'", converted)
        converted = re.sub(r'\bIN\s*\(\s*"([^"]+)"\s*\)', r"IN ('\1')", converted, flags=re.IGNORECASE)

        # Convert CASE THEN/ELSE double-quoted strings to single quotes
        # THEN "text" → THEN 'text', ELSE "text" → ELSE 'text'
        converted = re.sub(r'\bTHEN\s+"([^"]+)"', r"THEN '\1'", converted, flags=re.IGNORECASE)
        converted = re.sub(r'\bELSE\s+"([^"]+)"', r"ELSE '\1'", converted, flags=re.IGNORECASE)

        # Convert function argument double-quoted strings to single quotes
        # Common in SPLIT_PART, CONCAT, etc.: func(..., "delimiter", ...) → func(..., 'delimiter', ...)
        # Match: comma followed by whitespace, double-quoted string, optional whitespace, comma or closing paren
        converted = re.sub(r',(\s*)"([^"]+)"(\s*[,)])', r",\1'\2'\3", converted)

        # Convert MySQL FIELD() function to PostgreSQL CASE WHEN
        # MySQL: ORDER BY FIELD(column, 'val1', 'val2', 'val3')
        # PostgreSQL: ORDER BY CASE column WHEN 'val1' THEN 1 WHEN 'val2' THEN 2 WHEN 'val3' THEN 3 END
        def convert_field_function(match):
            column = match.group(1).strip()
            values_str = match.group(2).strip()
            # Split values by comma, handle quoted strings
            values = [v.strip() for v in re.findall(r"'[^']*'|[^,]+", values_str)]

            when_clauses = []
            for i, value in enumerate(values, start=1):
                when_clauses.append(f"WHEN {value} THEN {i}")

            return f"CASE {column} {' '.join(when_clauses)} END"

        converted = re.sub(
            r'\bFIELD\s*\(\s*([^,]+)\s*,\s*([^)]+)\s*\)',
            convert_field_function,
            converted,
            flags=re.IGNORECASE
        )

        # Convert double-quoted literals inside CONCAT to single quotes
        # CONCAT(..., "(text)", ...) → CONCAT(..., '(text)', ...)
        def replace_concat_strings(match):
            content = match.group(0)
            # Replace double-quoted strings inside CONCAT with single quotes
            content = re.sub(r'"([^"]+)"', r"'\1'", content)
            return content

        converted = re.sub(r'\bCONCAT\s*\([^)]+\)', replace_concat_strings, converted, flags=re.IGNORECASE)

        # Convert THEN/ELSE double-quoted strings to single quotes
        # THEN "text" → THEN 'text', ELSE "text" → ELSE 'text'
        converted = re.sub(r'\b(THEN|ELSE)\s+"([^"]+)"', r"\1 '\2'", converted, flags=re.IGNORECASE)

        # Convert SELECT "string literal" AS to SELECT 'string literal' AS
        # Only convert if string contains spaces (indicates literal not identifier)
        converted = re.sub(r'\bSELECT\s+"([^"]*\s[^"]*)"(\s+AS\s+)', r"SELECT '\1'\2", converted, flags=re.IGNORECASE)

        # Convert concatenation "string literal" to 'string literal'
        # Pattern: || "text" becomes || 'text'
        converted = re.sub(r'\|\|\s*"([^"]+)"', r"|| '\1'", converted)

        # Fix EXISTS(SELECT COUNT(...)) = 0/1 patterns that cause boolean = integer error
        # MySQL: EXISTS(SELECT COUNT(...)) = 0 returns true/false, PostgreSQL: EXISTS returns boolean, can't compare to int
        # Pattern 1: EXISTS(SELECT COUNT(...) FROM ...) = 0 → NOT EXISTS(SELECT 1 FROM ...)
        converted = re.sub(
            r'EXISTS\s*\(\s*SELECT\s+COUNT\s*\([^)]*\)\s+FROM\s+([^)]+)\)\s*=\s*0',
            r'NOT EXISTS(SELECT 1 FROM \1)',
            converted,
            flags=re.IGNORECASE
        )
        # Pattern 2: EXISTS(SELECT COUNT(...) FROM ...) = 1 → EXISTS(SELECT 1 FROM ...)
        converted = re.sub(
            r'EXISTS\s*\(\s*SELECT\s+COUNT\s*\([^)]*\)\s+FROM\s+([^)]+)\)\s*=\s*1',
            r'EXISTS(SELECT 1 FROM \1)',
            converted,
            flags=re.IGNORECASE
        )

        # Ambiguous alias fix DISABLED - causes incorrect _computed suffix duplication
        # The pattern was too greedy and matched cases where aliases are reused across CTEs
        # Example issue: adoption_pct → adoption_pct_computed → adoption_pct_computed_computed
        # If specific ambiguous column errors appear, handle them case-by-case

        # Fix mixed-type CASE expressions (TEXT and NUMERIC)
        # PostgreSQL: all CASE branches must be same type
        # Pattern: CASE with THEN 'string' and THEN numeric/arithmetic
        # Solution: Cast arithmetic branches to ::TEXT
        # Example: THEN value/60 ELSE 'No data' → THEN (value/60)::TEXT ELSE 'No data'
        def cast_numeric_then_clause(match):
            # Match captures: group(1) = THEN expression, group(2) = ELSE string
            then_expr = match.group(1).strip()
            else_str = match.group(2)
            # If expression contains division or CAST/NULLIF (likely numeric), wrap in ()::TEXT
            if '/' in then_expr or 'CAST' in then_expr.upper() or 'NULLIF' in then_expr.upper():
                # Wrap in parens if not already wrapped
                if not (then_expr.startswith('(') and then_expr.endswith(')')):
                    then_expr = f'({then_expr})'
                return f'THEN {then_expr}::TEXT ELSE \'{else_str}\''
            return match.group(0)

        # Match: THEN <numeric_expr> ELSE 'string_literal'
        # Negative lookahead (?!.*\bWHEN\b) ensures we don't match across WHEN boundaries
        converted = re.sub(
            r'\bTHEN\s+((?!.*\bWHEN\b).+?)\s+ELSE\s+\'([^\']+)\'',
            cast_numeric_then_clause,
            converted,
            flags=re.IGNORECASE | re.DOTALL
        )

        # Convert SUBSTRING_INDEX to PostgreSQL equivalent
        # MySQL: SUBSTRING_INDEX(str, delim, count) - get substring before/after delimiter occurrence
        # PostgreSQL: SPLIT_PART for positive count, REVERSE+SPLIT_PART for negative count
        def convert_substring_index(match):
            str_arg = match.group(1).strip()
            delim_arg = match.group(2).strip()
            count_arg = match.group(3).strip()

            # Check if count is negative (get from end)
            if count_arg.startswith('-'):
                # Negative count: get from end
                # SUBSTRING_INDEX(str, delim, -1) → REVERSE(SPLIT_PART(REVERSE(str), delim, 1))
                positive_count = count_arg[1:]  # Remove minus sign
                return f'REVERSE(SPLIT_PART(REVERSE({str_arg}), {delim_arg}, {positive_count}))'
            else:
                # Positive count: get from beginning
                # SUBSTRING_INDEX(str, delim, 1) → SPLIT_PART(str, delim, 1)
                return f'SPLIT_PART({str_arg}, {delim_arg}, {count_arg})'

        # Match: SUBSTRING_INDEX(str, delim, count)
        converted = re.sub(
            r'\bSUBSTRING_INDEX\s*\(\s*([^,]+),\s*([^,]+),\s*([^)]+)\)',
            convert_substring_index,
            converted,
            flags=re.IGNORECASE
        )

        # Fix STRING_AGG double-quoted separator (from sqlglot GROUP_CONCAT conversion)
        # PostgreSQL: STRING_AGG(expr, "sep") → STRING_AGG(expr, 'sep')
        # sqlglot converts GROUP_CONCAT but keeps separator quoting from source
        # Use greedy match to handle expressions with internal commas
        converted = re.sub(
            r'\bSTRING_AGG\s*\((.+),\s*"([^"]*)"\s*\)',
            r"STRING_AGG(\1, '\2')",
            converted,
            flags=re.IGNORECASE
        )

        # Fix HAVING clause with column alias - PostgreSQL doesn't allow aliases in HAVING
        # MySQL: HAVING `alias` IS NOT NULL
        # PostgreSQL: Remove HAVING on aggregate NULL checks (naturally filtered)
        # Pattern: HAVING NOT "alias" IS NULL or HAVING "alias" IS NOT NULL
        converted = re.sub(
            r'\bHAVING\s+(?:NOT\s+)?"([^"]+)"\s+IS\s+(?:NOT\s+)?NULL',
            '',
            converted,
            flags=re.IGNORECASE
        )

        # Fix ambiguous identifier in GROUP BY when alias matches column name
        # PostgreSQL can't distinguish between alias and column with same name
        # Keep GROUP BY as-is for now - dashboards using positional (1,2,3) work fine
        # Dashboards using aliases may have ambiguity - fix at dashboard level
        def quote_group_by_identifiers(sql_text):
            def quote_group_by_item(match):
                prefix = match.group(1)  # "GROUP BY "
                items_str = match.group(2)  # rest of clause

                # Keep positional numbers (1,2,3) as-is since they work perfectly
                # Keep complex expressions (table.col) as-is
                # Only quote simple identifiers for case-sensitivity
                items = []
                for item in items_str.split(','):
                    item = item.strip()
                    if not item or item.isdigit() or '.' in item:
                        items.append(item)
                    elif re.match(r'^\w+$', item):
                        items.append(f'"{item}"')
                    else:
                        items.append(item)

                return prefix + ', '.join(items)

            sql_text = re.sub(
                r'\b(GROUP\s+BY\s+)([^;]*?)(?=\s*(?:HAVING|ORDER|LIMIT|$))',
                quote_group_by_item,
                sql_text,
                flags=re.IGNORECASE
            )
            return sql_text

        converted = quote_group_by_identifiers(converted)

        # Quote capitalized identifiers in ORDER BY (same issue as GROUP BY)
        # Pattern: ORDER BY <Capitalized_word> - quote to match case-sensitive alias
        def quote_order_by_identifiers(sql_text):
            def quote_order_by_item(match):
                prefix = match.group(1)  # "ORDER BY "
                items_str = match.group(2)  # rest before DESC/ASC/NULLS/end

                # Split by comma
                items = []
                for item in items_str.split(','):
                    item = item.strip()
                    # Match: bare capitalized word (not quoted, not numeric)
                    word_match = re.match(r'^([A-Z]\w*)$', item)
                    if word_match and not item.isdigit():
                        items.append(f'"{item}"')
                    elif item and not item.startswith('"'):
                        items.append(item)
                    else:
                        items.append(item)

                return prefix + ', '.join(items)

            # Match ORDER BY clause up to DESC/ASC/NULLS or next keyword
            sql_text = re.sub(
                r'\b(ORDER\s+BY\s+)([^;]+?)(?=\s+(?:DESC|ASC|NULLS|LIMIT|$))',
                quote_order_by_item,
                sql_text,
                flags=re.IGNORECASE
            )
            return sql_text

        converted = quote_order_by_identifiers(converted)

        # Quote capitalized bare identifiers in SELECT column lists
        # Pattern: SELECT ..., Capitalized, ... (not after AS, not SQL keywords)
        # Must match column references that correspond to quoted aliases from subqueries
        def quote_select_columns(sql_text):
            # Match SELECT clause, extract column list
            def quote_columns_in_select(match):
                select_keyword = match.group(1)  # "SELECT " or "SELECT DISTINCT " etc
                columns_clause = match.group(2)  # column list

                # Split by comma, quote capitalized bare identifiers
                columns = []
                for col in columns_clause.split(','):
                    col = col.strip()
                    # Skip if already quoted, contains operators/functions, or is *
                    if col.startswith('"') or col == '*' or '(' in col or '.' in col or 'AS ' in col.upper():
                        columns.append(col)
                    # Quote capitalized bare word (e.g., Activity, Details, Name)
                    elif re.match(r'^[A-Z]\w*$', col):
                        columns.append(f'"{col}"')
                    else:
                        columns.append(col)

                return select_keyword + ', '.join(columns)

            # Match SELECT column list (from SELECT to FROM)
            sql_text = re.sub(
                r'\b(SELECT(?:\s+DISTINCT)?\s+)(.*?)(?=\s+FROM\b)',
                quote_columns_in_select,
                sql_text,
                flags=re.IGNORECASE
            )
            return sql_text

        converted = quote_select_columns(converted)

        # Quote table.Column references where Column is capitalized
        # Pattern: table.Date → table."Date" (prevents case-folding in JOIN/WHERE clauses)
        # Match: word.Capitalized (not followed by opening paren for functions)
        converted = re.sub(r'\.([A-Z]\w*)\b(?!\s*\()', r'."\1"', converted)

        # Quote bare capitalized identifiers in CASE WHEN clauses
        # WHEN Activity = 'value' → WHEN "Activity" = 'value'
        # Match WHEN case-insensitive, but identifier must start with uppercase
        converted = re.sub(
            r'\b(?:WHEN|when)\s+([A-Z]\w*)\s*(=|<>|!=|>|<|>=|<=|IN|LIKE|IS)',
            r'WHEN "\1" \2',
            converted
        )

        # Quote bare capitalized identifiers in WHERE clauses
        # WHERE Date > '2020' → WHERE "Date" > '2020'
        # Match WHERE case-insensitive, but identifier must start with uppercase
        converted = re.sub(
            r'\b(?:WHERE|where)\s+([A-Z]\w*)\s*(=|<>|!=|>|<|>=|<=|IN|LIKE|IS)',
            r'WHERE "\1" \2',
            converted
        )

        # Fix text column comparisons to integers
        # MySQL allows implicit conversion, PostgreSQL doesn't
        # issue_key <> 0 → issue_key <> '0' (text column needs text literal)
        converted = re.sub(
            r'\b([a-z_]+)\s*(=|<>|!=)\s*0\b',
            r"\1 \2 '0'",
            converted
        )

        # Protect equality comparisons with Grafana variables from empty values
        # connection_id = ${var} → ('${var}' = '' OR connection_id::text = '${var}')
        # table.connection_id = ${var} → ('${var}' = '' OR table.connection_id::text = '${var}')
        # When variable empty, condition becomes TRUE (no filter), when set, applies filter
        # Cast both sides to text to handle integer/text columns uniformly
        def protect_equality_variable(match):
            column = match.group(1)
            operator = match.group(2)
            var = match.group(3)
            return f"('{var}' = '' OR {column}::text {operator} '{var}')"

        # Match: column = ${var} or table.column = ${var}
        # Capture optional table prefix with (?:\.\w+)?
        converted = re.sub(
            r'(\w+(?:\.\w+)?)\s*(=|<>|!=)\s*(\$\{[^}]+\}|\$[a-z_][a-z0-9_]*)',
            protect_equality_variable,
            converted
        )

        # PostgreSQL rejects IN () when Grafana variables are empty
        # Use = ANY(ARRAY[...]) instead of IN (...) because empty array is valid SQL
        # When empty: = ANY(ARRAY[]::text[]) is always false
        # When multi-value: = ANY(ARRAY['val1','val2']::text[]) filters correctly
        def format_grafana_variable(var):
            """Extract variable name and return (var_formatted, var_check) for Grafana expansion."""
            if var.startswith('${') and var.endswith('}'):
                var_name = var[2:-1]
                if ':' not in var_name:
                    return f'${{{var_name}:singlequote}}', f'${{{var_name}:csv}}'
                return var, var
            # Bare $var form
            var_name = var[1:]
            return f'${{{var_name}:singlequote}}', f'${{{var_name}:csv}}'

        def protect_in_clause_with_cast(match):
            column = match.group(1)
            var = match.group(2)
            if '::' not in column:
                column = f'{column}::text'
            var_formatted, var_check = format_grafana_variable(var)
            return f"('{var_check}' = '' OR {column} = ANY(ARRAY[{var_formatted}]::text[]))"

        def protect_function_in_clause(match):
            func_expr = match.group(1)
            var = match.group(2)
            var_formatted, var_check = format_grafana_variable(var)
            return f"('{var_check}' = '' OR {func_expr} = ANY(ARRAY[{var_formatted}]::text[]))"

        converted = re.sub(
            r'([\w.]+(?:::\w+)?)\s+IN\s*\(\s*(\$\{[^}]+\}|\$(?!__)[a-z_][a-z0-9_]*)\s*\)',
            protect_in_clause_with_cast,
            converted,
            flags=re.IGNORECASE
        )

        converted = re.sub(
            r'(SPLIT_PART\([^)]+\))\s+IN\s*\(\s*(\$\{[^}]+\}|\$(?!__)[a-z_][a-z0-9_]*)\s*\)',
            protect_function_in_clause,
            converted,
            flags=re.IGNORECASE
        )

        return converted
    except Exception as e:
        # Fallback to regex-based conversion for complex cases
        print(f"  [WARN] sqlglot failed, using regex fallback: {str(e)[:100]}")
        fallback = regex_fallback_conversion(sql)
        fallback = restore_grafana_variables(fallback, placeholders)
        fallback = restore_string_literals(fallback, string_literals)
        return fallback


def protect_string_literals(sql_text: str) -> tuple[str, dict]:
    """
    Replace all string literals with placeholders to protect them from conversion.
    Returns modified SQL and mapping of placeholders to original strings.
    Handles SQL comments (-- and /* */) to avoid treating quotes in comments as string delimiters.
    """
    literals = {}
    counter = 0
    result = []
    i = 0

    while i < len(sql_text):
        # Skip single-line comments (-- comment)
        if i < len(sql_text) - 1 and sql_text[i:i+2] == '--':
            # Find end of line
            end = sql_text.find('\n', i)
            if end == -1:
                result.append(sql_text[i:])
                break
            else:
                result.append(sql_text[i:end+1])
                i = end + 1
            continue

        # Skip multi-line comments (/* comment */)
        if i < len(sql_text) - 1 and sql_text[i:i+2] == '/*':
            end = sql_text.find('*/', i + 2)
            if end == -1:
                result.append(sql_text[i:])
                break
            else:
                result.append(sql_text[i:end+2])
                i = end + 2
            continue

        # Process string literals
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


def convert_mysql_date_formats(sql: str) -> str:
    """
    Convert MySQL date format specifiers to PostgreSQL in TO_CHAR/TO_DATE calls.
    Operates character-by-character within format strings.
    Step 1 of the conversion fixes.
    """
    # MySQL -> PostgreSQL format mapping
    format_map = {
        '%Y': 'YYYY',  # 4-digit year
        '%y': 'YY',    # 2-digit year
        '%X': 'IYYY',  # ISO year
        '%V': 'IW',    # ISO week
        '%m': 'MM',    # month number
        '%M': 'FMMonth',  # full month name
        '%d': 'DD',    # day of month
        '%W': 'FMDay', # full weekday name
        '%w': 'D',     # day of week number
        '%u': 'IW',    # ISO week
        '%H': 'HH24',  # 24-hour
        '%i': 'MI',    # minutes
        '%s': 'SS'     # seconds
    }

    def convert_format_string(format_str):
        """Convert a single format string from MySQL to PostgreSQL."""
        # Skip if already PostgreSQL format (contains YYYY, MM, DD without %)
        if not '%' in format_str and any(pg in format_str for pg in ['YYYY', 'MM', 'DD', 'HH24']):
            return format_str

        result = []
        i = 0
        while i < len(format_str):
            if format_str[i] == '%' and i + 1 < len(format_str):
                # Try to match %X format specifier
                two_char = format_str[i:i+2]
                if two_char in format_map:
                    result.append(format_map[two_char])
                    i += 2
                else:
                    # Unknown specifier, keep as-is
                    result.append(format_str[i])
                    i += 1
            else:
                result.append(format_str[i])
                i += 1
        return ''.join(result)

    # Find and convert TO_CHAR(..., 'format') and TO_DATE(..., 'format')
    def replacer(match):
        func_name = match.group(1)
        first_arg = match.group(2)
        format_str = match.group(3)
        converted_format = convert_format_string(format_str)
        return f"{func_name}({first_arg}, '{converted_format}')"

    # Pattern matches TO_CHAR(expr, 'format') or TO_DATE(expr, 'format')
    # Use non-greedy matching for first argument
    sql = re.sub(
        r'\b(TO_CHAR|TO_DATE)\s*\(\s*([^,]+?)\s*,\s*\'([^\']+)\'\s*\)',
        replacer,
        sql,
        flags=re.IGNORECASE
    )

    return sql


def fix_timefilter_aggregate_macro(sql: str) -> str:
    """
    Fix $__timeFilter(MAX/MIN(...)) macro expansion that creates MAX(boolean).
    Converts: $__timeFilter(MAX(col)) -> MAX(col) BETWEEN $__timeFrom() AND $__timeTo()
    Step 5 of the conversion fixes.
    Must run AFTER restore_grafana_macros() to match lowercase $__timeFilter.
    """
    # Simple pattern for $__timeFilter(MAX(col)) or $__timeFilter(MIN(col))
    # Handles simple column references
    sql = re.sub(
        r'\$__timeFilter\s*\(\s*(MAX|MIN)\s*\(([^)]+)\)\s*\)',
        r'\1(\2) BETWEEN $__timeFrom() AND $__timeTo()',
        sql,
        flags=re.IGNORECASE
    )

    # For nested expressions like $__timeFilter(MAX(CAST(col AS DATE))),
    # use balanced paren matching
    pattern = r'\$__timeFilter\s*\('
    result = []
    i = 0
    while i < len(sql):
        match = re.match(pattern, sql[i:], re.IGNORECASE)
        if match:
            start = i + match.end()
            # Check if next token is MAX or MIN
            agg_match = re.match(r'\s*(MAX|MIN)\s*\(', sql[start:], re.IGNORECASE)
            if agg_match:
                agg_func = agg_match.group(1).upper()
                agg_start = start + agg_match.end()
                # Find matching closing paren for MAX/MIN
                depth = 1
                j = agg_start
                while j < len(sql) and depth > 0:
                    if sql[j] == '(':
                        depth += 1
                    elif sql[j] == ')':
                        depth -= 1
                    j += 1
                if depth == 0:
                    # Found closing paren for MAX/MIN
                    agg_arg = sql[agg_start:j-1]
                    # Now find closing paren for $__timeFilter
                    if j < len(sql) and sql[j] == ')':
                        # Replace with aggregate BETWEEN pattern
                        result.append(f"{agg_func}({agg_arg}) BETWEEN $__timeFrom() AND $__timeTo()")
                        i = j + 1
                        continue
            # Not a MAX/MIN pattern, keep original
            result.append(sql[i])
            i += 1
        else:
            result.append(sql[i])
            i += 1

    return ''.join(result)


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

    # DAYOFMONTH() -> EXTRACT(DAY FROM ...) (no underscore variant)
    sql = replace_function_with_balanced_parens(
        sql,
        r'\bDAYOFMONTH\s*\(',
        lambda arg: f'EXTRACT(DAY FROM {arg})'
    )

    # DAY_OF_MONTH() -> EXTRACT(DAY FROM ...) (with underscore variant)
    sql = replace_function_with_balanced_parens(
        sql,
        r'\bDAY_OF_MONTH\s*\(',
        lambda arg: f'EXTRACT(DAY FROM {arg})'
    )

    # DAY() -> EXTRACT(DAY FROM ...)
    sql = replace_function_with_balanced_parens(
        sql,
        r'\bDAY\s*\(',
        lambda arg: f'EXTRACT(DAY FROM {arg})'
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

    # NUMBER_TO_STR -> ROUND + cast to text
    # MySQL custom: NUMBER_TO_STR(value, decimals)
    # PostgreSQL: ROUND(value, decimals)::TEXT
    sql = replace_function_with_balanced_parens(
        sql,
        r'\bNUMBER_TO_STR\s*\(',
        lambda arg: f'ROUND({arg})::TEXT'
    )

    # FIND_IN_SET -> ANY(string_to_array())
    # MySQL: FIND_IN_SET(value, csv_string) > 0
    # PostgreSQL: value = ANY(string_to_array(csv_string, ','))
    # Iteratively replace innermost FIND_IN_SET first
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

    # Step 3: Fix MySQL-specific type casts
    sql = re.sub(r'\bUBIGINT\b', 'BIGINT', sql, flags=re.IGNORECASE)
    sql = re.sub(r'\bAS\s+UNSIGNED\b', 'AS BIGINT', sql, flags=re.IGNORECASE)
    sql = re.sub(r'\bAS\s+SIGNED\b', 'AS BIGINT', sql, flags=re.IGNORECASE)

    # Step 4: YEARWEEK() conversion
    # YEARWEEK(col, 1) -> (EXTRACT(ISOYEAR FROM col) * 100 + EXTRACT(WEEK FROM col))::int
    sql = re.sub(
        r'\bYEARWEEK\s*\(\s*([^,]+?)\s*,\s*\d+\s*\)',
        r'(EXTRACT(ISOYEAR FROM \1) * 100 + EXTRACT(WEEK FROM \1))::int',
        sql,
        flags=re.IGNORECASE
    )

    return sql


def protect_grafana_variables(sql: str) -> tuple:
    """
    Replace Grafana template variables with safe placeholders before sqlglot.
    sqlglot uppercases bare variables like $interval → $INTERVAL, breaking Grafana interpolation.
    Protects both ${var} and $var (but not $__macros).
    Returns: (modified_sql, dict of placeholders)
    """
    placeholders = {}
    counter = 0

    def replacer(match):
        nonlocal counter
        placeholder = f'GRAFANA_PLACEHOLDER_{counter}_GRAFANA'
        placeholders[placeholder] = match.group(0)
        counter += 1
        return placeholder

    # Protect ${variable} or ${variable:format}
    protected_sql = re.sub(r'\$\{([^}]+)\}', replacer, sql)

    # Protect bare $variable (but not $__macros like $__timeFilter)
    protected_sql = re.sub(r'\$(?!__)[a-zA-Z_][a-zA-Z0-9_]*', replacer, protected_sql)

    return protected_sql, placeholders


def restore_grafana_variables(sql: str, placeholders: dict) -> str:
    """
    Restore Grafana template variables from safe placeholders.
    """
    for placeholder, original in placeholders.items():
        sql = sql.replace(placeholder, original)

    return sql


def restore_grafana_macros(sql: str) -> str:
    """
    Restore Grafana macros to lowercase after sqlglot uppercases them.
    Grafana expects: $__timeFilter(), $__timeFrom(), $__timeTo()
    """
    sql = re.sub(r'\$__TIMEFILTER', '$__timeFilter', sql)
    sql = re.sub(r'\$__TIMEFROM', '$__timeFrom', sql)
    sql = re.sub(r'\$__TIMETO', '$__timeTo', sql)

    # Step 2: Fix WEEKDAY/INTERVAL conversion (broadened pattern)
    # Handles: day - INTERVAL 'WEEKDAY DAY', metric_date - INTERVAL 'WEEKDAY DAY',
    #          TO_TIMESTAMP(...) - INTERVAL 'WEEKDAY DAY', etc.
    # Find INTERVAL 'WEEKDAY DAY' and walk backwards to find expression
    pattern = r"INTERVAL\s+['\"]WEEKDAY\s+DAY['\"]"
    result = []
    i = 0

    while i < len(sql):
        match = re.search(pattern, sql[i:], re.IGNORECASE)
        if not match:
            result.append(sql[i:])
            break

        # Found INTERVAL 'WEEKDAY DAY' at position i + match.start()
        interval_start = i + match.start()

        # Walk backwards to find the subtraction operator and LHS expression
        # Look for ' - ' before the INTERVAL
        minus_pos = None
        for j in range(interval_start - 1, max(0, interval_start - 50), -1):
            if sql[j:j+3] == ' - ' or (j > 0 and sql[j-1:j+2] == ' - '):
                minus_pos = j
                break

        if minus_pos is not None:
            # Walk backwards from minus to find start of expression
            # Stop at keywords, commas, or opening parens at depth 0
            expr_start = 0
            depth = 0
            for j in range(minus_pos - 1, -1, -1):
                if sql[j] == ')':
                    depth += 1
                elif sql[j] == '(':
                    depth -= 1
                elif depth == 0 and sql[j] in (',', '\n'):
                    expr_start = j + 1
                    break
                elif depth == 0 and j >= 8:
                    # Check for SQL keywords - need to look for GROUP BY specifically
                    upper_substr = sql[max(0, j-8):j+1].upper()
                    for kw in ['GROUP BY ', 'ORDER BY ', 'SELECT ', 'WHERE ', 'AND ', 'WHEN ', 'BETWEEN ']:
                        kw_pos = upper_substr.rfind(kw)
                        if kw_pos >= 0:
                            # Found keyword, expr starts after it
                            expr_start = max(0, j-8) + kw_pos + len(kw)
                            break
                    if expr_start > 0:
                        break

            # Extract the expression
            expr = sql[expr_start:minus_pos].strip()

            # Append everything before expr_start
            result.append(sql[i:expr_start])

            # Build replacement
            replacement = f"{expr} - (EXTRACT(ISODOW FROM {expr}) - 1) * INTERVAL '1 day'"
            result.append(replacement)

            # Skip past the INTERVAL 'WEEKDAY DAY'
            # match.end() is relative to sql[i:], so absolute position is i + match.end()
            i = i + match.end()
        else:
            # Could not find minus operator, keep original
            result.append(match.group(0))
            i = i + match.end()

    sql = ''.join(result)

    # Also handle: expr + INTERVAL '6 DAY' pattern (week-end calculation)
    # Convert: expr - INTERVAL 'WEEKDAY DAY' + INTERVAL '6 DAY'
    # But this is handled naturally by the above since we only replace the WEEKDAY part

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
    # Use balanced paren parsing to handle nested expressions
    def replace_if(args):
        # Parse comma-separated args manually (can't use split because of nested parens)
        parts = []
        depth = 0
        current = []
        for char in args:
            if char == ',' and depth == 0:
                parts.append(''.join(current).strip())
                current = []
            else:
                if char == '(':
                    depth += 1
                elif char == ')':
                    depth -= 1
                current.append(char)
        if current:
            parts.append(''.join(current).strip())

        if len(parts) == 3:
            return f'CASE WHEN {parts[0]} THEN {parts[1]} ELSE {parts[2]} END'
        return f'IF({args})'  # Fallback if parse fails

    sql = replace_function_with_balanced_parens(
        sql,
        r'\bIF\s*\(',
        replace_if
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
            # Convert query and definition fields (dashboard variables)
            elif key in ("query", "definition") and isinstance(value, str) and ("SELECT" in value.upper() or "CAST" in value.upper()):
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

        # Title kept as-is (no PostgreSQL suffix needed)

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
