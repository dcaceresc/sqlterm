package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (a *adapter) Catalog(ctx context.Context, c *sql.Conn) ([]Object, error) {
	var q string
	switch a.engine {
	case "sqlserver":
		q = `SELECT s.name,o.name,CASE o.type WHEN 'U' THEN 'table' WHEN 'V' THEN 'view' WHEN 'P' THEN 'procedure' ELSE 'function' END,CAST(o.object_id AS varchar(30)),'',CONVERT(bit,CASE o.type WHEN 'FN' THEN 1 ELSE 0 END) FROM sys.objects o JOIN sys.schemas s ON s.schema_id=o.schema_id WHERE o.type IN ('U','V','P','FN','IF','TF') AND o.is_ms_shipped=0 ORDER BY s.name,o.name`
	case "mysql":
		q = `SELECT TABLE_SCHEMA,TABLE_NAME,CASE TABLE_TYPE WHEN 'VIEW' THEN 'view' ELSE 'table' END,TABLE_NAME,'',FALSE FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() UNION ALL SELECT ROUTINE_SCHEMA,ROUTINE_NAME,LOWER(ROUTINE_TYPE),ROUTINE_NAME,COALESCE((SELECT GROUP_CONCAT(CONCAT(COALESCE(p.PARAMETER_MODE,''),' ',p.PARAMETER_NAME,' ',p.DTD_IDENTIFIER) ORDER BY p.ORDINAL_POSITION SEPARATOR ', ') FROM information_schema.PARAMETERS p WHERE p.SPECIFIC_SCHEMA=r.ROUTINE_SCHEMA AND p.SPECIFIC_NAME=r.ROUTINE_NAME AND p.ORDINAL_POSITION>0),''),FALSE FROM information_schema.ROUTINES r WHERE ROUTINE_SCHEMA=DATABASE() ORDER BY 1,2`
	default:
		q = `SELECT n.nspname,c.relname,CASE c.relkind WHEN 'v' THEN 'view' WHEN 'm' THEN 'view' ELSE 'table' END,c.oid::text,'',false FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('r','p','v','m','f') AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%' UNION ALL SELECT n.nspname,p.proname,CASE p.prokind WHEN 'p' THEN 'procedure' ELSE 'function' END,p.oid::text,pg_get_function_identity_arguments(p.oid),false FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE p.prokind IN ('p','f') AND n.nspname NOT IN ('pg_catalog','information_schema') ORDER BY 1,2,5`
	}
	r, e := c.QueryContext(ctx, q)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	var out []Object
	for r.Next() {
		var o Object
		if e = r.Scan(&o.Schema, &o.Name, &o.Kind, &o.ID, &o.Signature, &o.Scalar); e != nil {
			return nil, e
		}
		out = append(out, o)
	}
	if e = r.Err(); e != nil {
		return nil, e
	}
	if e = r.Close(); e != nil {
		return nil, e
	}
	if a.engine == "sqlserver" && len(out) > 0 {
		if e = sqlServerSignatures(ctx, c, out); e != nil {
			return nil, e
		}
	}
	return out, nil
}

// Assemble signatures in Go so older SQL Server versions need no aggregate
// string functions or XML encoding. Close object rows before reusing the session.
func sqlServerSignatures(ctx context.Context, c *sql.Conn, objects []Object) error {
	indexes := make(map[string]int)
	for i, o := range objects {
		if o.Kind == "procedure" || o.Kind == "function" {
			indexes[o.ID] = i
		}
	}
	if len(indexes) == 0 {
		return nil
	}
	rows, err := c.QueryContext(ctx, `SELECT CONVERT(varchar(30),p.object_id),p.name,COALESCE(TYPE_NAME(p.user_type_id),'?') FROM sys.parameters p JOIN sys.objects o ON o.object_id=p.object_id WHERE p.parameter_id>0 AND o.type IN ('P','FN','IF','TF') AND o.is_ms_shipped=0 ORDER BY p.object_id,p.parameter_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, typ string
		if err = rows.Scan(&id, &name, &typ); err != nil {
			return err
		}
		if i, ok := indexes[id]; ok {
			if objects[i].Signature != "" {
				objects[i].Signature += ", "
			}
			objects[i].Signature += name + " " + typ
		}
	}
	return rows.Err()
}
func (a *adapter) Definition(ctx context.Context, c *sql.Conn, o Object) (string, error) {
	var text sql.NullString
	switch a.engine {
	case "sqlserver":
		if o.Kind == "table" {
			return "SELECT TOP (1000) * FROM " + a.qualified(o) + ";", nil
		}
		e := c.QueryRowContext(ctx, `SELECT OBJECT_DEFINITION(CONVERT(int,@p1))`, o.ID).Scan(&text)
		if e != nil {
			return "", e
		}
	case "postgres":
		if o.Kind == "table" {
			return "SELECT * FROM " + a.qualified(o) + " LIMIT 1000;", nil
		}
		q := `SELECT pg_get_functiondef($1::oid)`
		if o.Kind == "view" {
			q = `SELECT pg_get_viewdef($1::oid,true)`
		}
		e := c.QueryRowContext(ctx, q, o.ID).Scan(&text)
		if e != nil {
			return "", e
		}
		if o.Kind == "view" && text.Valid {
			text.String = "CREATE OR REPLACE VIEW " + a.qualified(o) + " AS\n" + text.String
		}
	case "mysql":
		if o.Kind == "table" {
			return "SELECT * FROM " + a.qualified(o) + " LIMIT 1000;", nil
		}
		kind := strings.ToUpper(o.Kind)
		if kind != "PROCEDURE" && kind != "FUNCTION" && kind != "VIEW" {
			return "", unsupported(kind)
		}
		r, e := c.QueryContext(ctx, "SHOW CREATE "+kind+" "+a.qualified(o))
		if e != nil {
			return "", e
		}
		defer r.Close()
		cols, e := r.Columns()
		if e != nil {
			return "", e
		}
		if r.Next() {
			vals := make([]sql.NullString, len(cols))
			dest := make([]any, len(cols))
			for i := range vals {
				dest[i] = &vals[i]
			}
			if e = r.Scan(dest...); e != nil {
				return "", e
			}
			for i, col := range cols {
				if strings.HasPrefix(strings.ToLower(col), "create ") {
					text = vals[i]
				}
			}
		}
		if e = r.Err(); e != nil {
			return "", e
		}
	}
	if !text.Valid || text.String == "" {
		return "", fmt.Errorf("definition unavailable: check permissions or encryption")
	}
	return text.String, nil
}
func (a *adapter) Parameters(ctx context.Context, c *sql.Conn, o Object) ([]Parameter, error) {
	if o.Kind != "procedure" && o.Kind != "function" {
		return nil, nil
	}
	var q string
	var args []any
	switch a.engine {
	case "sqlserver":
		q = `SELECT p.name,CASE WHEN t.is_user_defined=1 THEN QUOTENAME(SCHEMA_NAME(t.schema_id))+'.'+QUOTENAME(t.name) ELSE QUOTENAME(t.name)+CASE WHEN t.name IN ('varchar','char','varbinary','binary') THEN '('+CASE WHEN p.max_length=-1 THEN 'max' ELSE CAST(p.max_length AS varchar(10)) END+')' WHEN t.name IN ('nvarchar','nchar') THEN '('+CASE WHEN p.max_length=-1 THEN 'max' ELSE CAST(p.max_length/2 AS varchar(10)) END+')' WHEN t.name IN ('decimal','numeric') THEN '('+CAST(p.precision AS varchar(10))+','+CAST(p.scale AS varchar(10))+')' WHEN t.name IN ('time','datetime2','datetimeoffset') THEN '('+CAST(p.scale AS varchar(10))+')' ELSE '' END END,CASE WHEN t.is_table_type=1 THEN 'TABLE' WHEN p.is_output=1 THEN 'INOUT' ELSE 'IN' END,CONVERT(bit,p.has_default_value) FROM sys.parameters p JOIN sys.types t ON t.user_type_id=p.user_type_id WHERE p.object_id=CONVERT(int,@p1) AND p.parameter_id>0 ORDER BY p.parameter_id`
		args = []any{o.ID}
	case "mysql":
		q = `SELECT PARAMETER_NAME,DTD_IDENTIFIER,PARAMETER_MODE,FALSE FROM information_schema.PARAMETERS WHERE SPECIFIC_SCHEMA=? AND SPECIFIC_NAME=? AND ORDINAL_POSITION>0 ORDER BY ORDINAL_POSITION`
		args = []any{o.Schema, o.Name}
	default:
		q = `SELECT COALESCE(NULLIF(p.proargnames[i],''), 'arg'||i),format_type(t,NULL),CASE COALESCE(p.proargmodes[i],'i') WHEN 'o' THEN 'OUT' WHEN 't' THEN 'OUT' WHEN 'b' THEN 'INOUT' WHEN 'v' THEN 'VARIADIC' ELSE 'IN' END, (COALESCE(p.proargmodes[i],'i') IN ('i','b','v') AND (SELECT count(*) FROM generate_series(1,i) j WHERE COALESCE(p.proargmodes[j],'i') IN ('i','b','v')) > p.pronargs-p.pronargdefaults) FROM pg_proc p CROSS JOIN LATERAL unnest(COALESCE(p.proallargtypes,p.proargtypes::oid[])) WITH ORDINALITY a(t,i) WHERE p.oid=$1::oid ORDER BY i`
		args = []any{o.ID}
	}
	r, e := c.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	var out []Parameter
	for r.Next() {
		var p Parameter
		if e = r.Scan(&p.Name, &p.Type, &p.Direction, &p.Default); e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	if e = r.Err(); e != nil {
		return nil, e
	}
	r.Close()
	if a.engine == "sqlserver" {
		if definition, err := a.Definition(ctx, c, o); err == nil {
			defaults := parameterDefaults(definition)
			for i := range out {
				out[i].Default = out[i].Default || defaults[strings.ToLower(out[i].Name)]
			}
		}
	}
	return out, nil
}
func (a *adapter) Columns(ctx context.Context, c *sql.Conn, o Object) ([]string, error) {
	switch a.engine {
	case "sqlserver":
		return queryStrings(ctx, c, `SELECT name FROM sys.columns WHERE object_id=CONVERT(int,@p1) ORDER BY column_id`, o.ID)
	case "mysql":
		return queryStrings(ctx, c, `SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? ORDER BY ORDINAL_POSITION`, o.Schema, o.Name)
	default:
		return queryStrings(ctx, c, `SELECT attname FROM pg_attribute WHERE attrelid=$1::oid AND attnum>0 AND NOT attisdropped ORDER BY attnum`, o.ID)
	}
}
func (a *adapter) Databases(ctx context.Context, c *sql.Conn) ([]string, error) {
	switch a.engine {
	case "sqlserver":
		return queryStrings(ctx, c, `SELECT name FROM sys.databases WHERE HAS_DBACCESS(name)=1 ORDER BY name`)
	case "mysql":
		return queryStrings(ctx, c, `SHOW DATABASES`)
	default:
		return queryStrings(ctx, c, `SELECT datname FROM pg_database WHERE datallowconn AND has_database_privilege(datname,'CONNECT') ORDER BY datname`)
	}
}
