package profiles

import (
	"errors"
	"fmt"

	"github.com/BurntSushi/toml"
)

// portableUseCaseLegacyV2 is the v2 use_case block, where primary was a single string.
type portableUseCaseLegacyV2 struct {
	Primary string   `toml:"primary,omitempty"`
	Tags    []string `toml:"tags,omitempty"`
}

// portableProfileLegacyV2 mirrors PortableProfile with a v2 use_case.
type portableProfileLegacyV2 struct {
	Name      string                  `toml:"name"`
	Backend   string                  `toml:"backend"`
	ModelHint string                  `toml:"model_hint,omitempty"`
	Args      []string                `toml:"args,omitempty"`
	Env       []PortableEnvVar        `toml:"env,omitempty"`
	UseCase   portableUseCaseLegacyV2 `toml:"use_case,omitempty"`
	Hardware  PortableHardware        `toml:"hardware,omitempty"`
}

// portableFileLegacyV2 is the top-level v2 portable TOML document.
type portableFileLegacyV2 struct {
	SchemaVersion int                       `toml:"schema_version"`
	Profiles      []portableProfileLegacyV2 `toml:"profiles"`
}

// errInvalidTOML marks a body that is not TOML at all, as opposed to TOML that
// says something this version does not accept. Callers wrap the two differently
// because a file path and a URL need different advice.
var errInvalidTOML = errors.New("invalid TOML")

// unsupportedSchemaError reports a schema_version this build cannot read.
type unsupportedSchemaError struct {
	Got int
}

func (e *unsupportedSchemaError) Error() string {
	return fmt.Sprintf("unsupported schema_version %d (expected %d)", e.Got, SchemaVersion)
}

// missingNameError reports a profile with no name; the name is its identity, so
// there is nothing sensible to import it as.
type missingNameError struct {
	Index int // 1-based, for humans
}

func (e *missingNameError) Error() string {
	return fmt.Sprintf("profile %d missing name", e.Index)
}

// parsePortable decodes a portable profile document, migrating schema version 2
// to the current version. It is the single place that knows the on-disk
// portable format; ReadPortable and Fetcher.FetchPortable both go through it so
// a new schema version is added once.
func parsePortable(body []byte) (*PortableFile, error) {
	// Peek at the version before committing to a shape.
	var vo struct {
		SchemaVersion int `toml:"schema_version"`
	}
	if err := toml.Unmarshal(body, &vo); err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidTOML, err)
	}

	var f *PortableFile
	switch vo.SchemaVersion {
	case SchemaVersion:
		var pf PortableFile
		if err := toml.Unmarshal(body, &pf); err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidTOML, err)
		}
		f = &pf
	case 2:
		var lf portableFileLegacyV2
		if err := toml.Unmarshal(body, &lf); err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidTOML, err)
		}
		f = migrateV2(lf)
	default:
		return nil, &unsupportedSchemaError{Got: vo.SchemaVersion}
	}

	for i, p := range f.Profiles {
		if p.Name == "" {
			return nil, &missingNameError{Index: i + 1}
		}
	}
	return f, nil
}

// migrateV2 lifts a v2 document to the current schema. The only difference is
// use_case.primary, which was a single string and is now a list.
func migrateV2(lf portableFileLegacyV2) *PortableFile {
	out := &PortableFile{
		SchemaVersion: SchemaVersion,
		Profiles:      make([]PortableProfile, len(lf.Profiles)),
	}
	for i, lp := range lf.Profiles {
		pp := PortableProfile{
			Name:      lp.Name,
			Backend:   lp.Backend,
			ModelHint: lp.ModelHint,
			Args:      lp.Args,
			Env:       lp.Env,
			Hardware:  lp.Hardware,
			UseCase:   PortableUseCase{Tags: lp.UseCase.Tags},
		}
		if lp.UseCase.Primary != "" {
			pp.UseCase.Primary = []string{lp.UseCase.Primary}
		}
		out.Profiles[i] = pp
	}
	return out
}
