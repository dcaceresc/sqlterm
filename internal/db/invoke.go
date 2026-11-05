package db

import (
	"fmt"
	"strings"
)

type Bound struct {
	SQL  string
	Args []any
}

func (a *adapter) Invoke(o Object, params []Parameter, values []Value) (Invocation, error) {
	inv := Invocation{}
	if len(params) != len(values) {
		return inv, fmt.Errorf("parameter count mismatch")
	}
	var parts, decls, outputs []string
	arg := 0
	omitted := false
	for i, p := range params {
		v := values[i]
		if v.Omit {
			if !p.Default {
				return inv, fmt.Errorf("%s has no known default", p.Name)
			}
			if a.engine == "sqlserver" {
				if o.Kind == "function" {
					parts = append(parts, "DEFAULT")
				}
				continue
			}
			omitted = true
			continue
		}
		if omitted && a.engine == "postgres" && p.Direction != "OUT" {
			return inv, fmt.Errorf("only trailing input parameters may be omitted")
		}
		var val any = v.Text
		if v.Null || p.Direction == "OUT" {
			val = nil
		}
		switch a.engine {
		case "sqlserver":
			if p.Direction == "TABLE" {
				return inv, fmt.Errorf("table-valued parameter %s requires a SQL script", p.Name)
			}
			arg++
			variable := fmt.Sprintf("@sqlterm_v_%d", i)
			decls = append(decls, fmt.Sprintf("DECLARE %s %s = @p%d;", variable, p.Type, arg))
			inv.Args = append(inv.Args, val)
			if o.Kind == "function" {
				parts = append(parts, variable)
			} else {
				part := p.Name + " = " + variable
				if p.Direction == "INOUT" || p.Direction == "OUT" {
					part += " OUTPUT"
					outputs = append(outputs, variable+" AS "+a.Quote(p.Name))
				}
				parts = append(parts, part)
			}
		case "postgres":
			if p.Direction == "OUT" {
				if o.Kind == "procedure" {
					parts = append(parts, "NULL")
				}
				continue
			}
			arg++
			prefix := ""
			if p.Direction == "VARIADIC" {
				prefix = "VARIADIC "
			}
			parts = append(parts, fmt.Sprintf("%s$%d::%s", prefix, arg, p.Type))
			inv.Args = append(inv.Args, val)
		case "mysql":
			if p.Direction == "OUT" || p.Direction == "INOUT" {
				variable := fmt.Sprintf("@sqlterm_out_%d", i)
				inv.Before = append(inv.Before, Bound{"SET " + variable + " = ?", []any{val}})
				parts = append(parts, variable)
				outputs = append(outputs, variable+" AS "+a.Quote(p.Name))
			} else {
				parts = append(parts, "?")
				inv.Args = append(inv.Args, val)
			}
		}
	}
	switch a.engine {
	case "sqlserver":
		if o.Kind == "function" {
			prefix := "SELECT * FROM "
			suffix := ";"
			if o.Scalar {
				prefix = "SELECT "
				suffix = " AS value;"
			}
			inv.SQL = strings.Join(decls, "\n") + "\n" + prefix + a.qualified(o) + "(" + strings.Join(parts, ", ") + ")" + suffix
		} else {
			outputs = append(outputs, "@sqlterm_return AS [RETURN_STATUS]")
			inv.SQL = "DECLARE @sqlterm_return int;\n" + strings.Join(decls, "\n") + "\nEXEC @sqlterm_return = " + a.qualified(o) + " " + strings.Join(parts, ", ") + ";\nSELECT " + strings.Join(outputs, ", ") + ";"
		}
	case "postgres":
		prefix := "SELECT * FROM "
		if o.Kind == "procedure" {
			prefix = "CALL "
		}
		inv.SQL = prefix + a.qualified(o) + "(" + strings.Join(parts, ", ") + ")"
	case "mysql":
		prefix := "CALL "
		if o.Kind == "function" {
			prefix = "SELECT "
		}
		inv.SQL = prefix + a.qualified(o) + "(" + strings.Join(parts, ", ") + ")"
		if len(outputs) > 0 {
			inv.After = "SELECT " + strings.Join(outputs, ", ")
		}
	}
	return inv, nil
}
