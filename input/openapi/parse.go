// Package openapi loads OpenAPI documents and converts them into the
// template-facing model in package spec.
//
// Parsing is delegated to libopenapi; nothing outside this package should depend
// on that library, so that the parser stays replaceable without touching
// templates or generators.
package openapi

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/rs/zerolog"

	"github.com/gofoji/foji/errs"
	"github.com/gofoji/foji/input"
	"github.com/gofoji/foji/input/openapi/spec"
)

// errNoModel indicates the document parsed but produced no OpenAPI v3 model.
const errNoModel = errs.Error("unable to build OpenAPI v3 model")

type (
	FileGroups []FileGroup
	FileGroup  []File
)

type File struct {
	Input input.File
	API   *spec.T
}

func Parse(_ context.Context, logger zerolog.Logger, inGroups []input.FileGroup) (FileGroups, error) {
	result := make(FileGroups, len(inGroups))

	for i, ff := range inGroups {
		var group FileGroup

		for _, f := range ff.Files {
			logger.Info().Msgf("Parsing swagger from: %s", f.Source)

			api, err := parseFile(logger, f)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f.Source, err)
			}

			group = append(group, File{Input: f, API: api})
		}

		result[i] = group
	}

	return result, nil
}

func parseFile(logger zerolog.Logger, f input.File) (*spec.T, error) {
	doc, err := libopenapi.NewDocumentWithConfiguration(f.Content, &datamodel.DocumentConfiguration{
		AllowFileReferences: true,
		// External refs are resolved relative to the document, not the process
		// working directory.
		BasePath:     filepath.Dir(f.Source),
		SpecFilePath: f.Source,
	})
	if err != nil {
		return nil, err
	}

	model, err := doc.BuildV3Model()
	if model == nil {
		if err == nil {
			err = errNoModel
		}

		return nil, err
	}

	// A document can build with non-fatal complaints — most often circular
	// references, which generation handles. Report them without refusing to
	// generate, since the previous loader accepted these documents.
	if err != nil {
		logger.Warn().Err(err).Msgf("Issues resolving %s", f.Source)
	}

	return newConverter().document(&model.Model), nil
}
