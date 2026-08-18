package output

import (
	"errors"
	"fmt"
	"strings"

	"github.com/codemodus/kace"
	"github.com/gofoji/plates"
	"github.com/rs/zerolog"

	"github.com/gofoji/foji/cfg"
	"github.com/gofoji/foji/input/sql"
	"github.com/gofoji/foji/stringlist"
)

const (
	SQLAll   = "SQLAll"
	SQLFiles = "SQLFiles"
	SQLFile  = "SQLFile"
	SQLQuery = "SQLQuery"
)

func HasSQLOutput(o cfg.Output) bool {
	return hasAnyOutput(o, SQLAll, SQLFiles, SQLFile, SQLQuery)
}

func SQL(p cfg.Process, fn cfg.FileHandler, l zerolog.Logger, fileGroups sql.FileGroups, simulate bool) error {
	base := SQLContext{
		Context:    NewContext(p, l),
		FileGroups: fileGroups,
	}

	g := newGen(p, fn, l, simulate)

	g.render(SQLAll, &base)

	for _, ff := range fileGroups {
		if g.err != nil {
			break
		}

		groupCtx := SQLFileGroupContext{
			SQLContext: base,
			Files:      ff,
		}

		g.render(SQLFiles, &groupCtx)

		for _, f := range ff {
			fileCtx := SQLFileContext{
				SQLContext: base,
				File:       f,
			}

			g.render(SQLFile, &fileCtx)

			for _, q := range f.Queries {
				queryCtx := SQLQueryContext{
					SQLContext: base,
					Query:      q,
				}

				g.render(SQLQuery, &queryCtx)
			}
		}
	}

	return g.err
}

//nolint:recvcheck
type SQLContext struct {
	Context
	sql.FileGroups
	Imports
}

type SQLFileGroupContext struct {
	SQLContext

	Files []sql.File
}

type SQLFileContext struct {
	SQLContext
	sql.File
}

type SQLQueryContext struct {
	SQLContext
	sql.Query
}

func (q SQLContext) Parameterize(cc sql.Params, format, pkg string) string {
	ss := make(stringlist.Strings, len(cc))

	for x := range cc {
		ss[x] = fmt.Sprintf(format, kace.Camel(cc[x].Name), q.GetType(cc[x], pkg))
	}

	return strings.Join(ss, ", ")
}

func (q SQLContext) GetType(c *sql.Param, pkg string) string {
	if c.Generated {
		return c.Type
	}

	return ResolveType(q.Maps, func(t string) string { return q.CheckPackage(t, pkg) }, c.Type, c.Nullable, c.Path())
}

var errMissingParam = errors.New("missing Param.Package")

// checkQueryPackages registers the import for every type referenced by qq,
// covering both the result type and the query parameters.
func checkQueryPackages(ii *Imports, pkg string, qq []sql.Query) {
	for _, q := range qq {
		ii.CheckPackage(q.Result.Type, pkg)
		checkParamPackages(ii, pkg, q.Params)
	}
}

// checkParamPackages registers the import for every type referenced by pp.
func checkParamPackages(ii *Imports, pkg string, pp sql.Params) {
	for _, p := range pp {
		ii.CheckPackage(p.Type, pkg)
	}
}

func (q *SQLContext) Init() error {
	name, ok := q.Params.HasString("Package")
	if !ok {
		return errMissingParam
	}

	for _, set := range q.FileGroups {
		for _, ff := range set {
			checkQueryPackages(&q.Imports, name, ff.Queries)
		}
	}

	return nil
}

func (q *SQLFileGroupContext) Init() error {
	name, ok := q.Params.HasString("Package")
	if !ok {
		return errMissingParam
	}

	for _, ff := range q.Files {
		checkQueryPackages(&q.Imports, name, ff.Queries)
	}

	return nil
}

func (q *SQLFileContext) Init(p *plates.Factory) error {
	name, ok := q.Params.HasString("Package")
	if !ok {
		return errMissingParam
	}

	if strings.Contains(name, "{{") {
		var err error

		name, err = p.From(name).To(q)
		if err != nil {
			return fmt.Errorf("mapping Package name:%w", err)
		}

		q.Params["Package"] = name
	}

	checkQueryPackages(&q.Imports, name, q.Queries)

	return nil
}

func (q *SQLQueryContext) Init() error {
	name, ok := q.Process.Params.HasString("Package")
	if !ok {
		return errMissingParam
	}

	// q.Params resolves to the embedded sql.Query params, not the process params.
	checkParamPackages(&q.Imports, name, q.Params)

	return nil
}
