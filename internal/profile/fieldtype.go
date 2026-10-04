package profile

// FieldType classifies how a field should be encoded on disk for a given
// slicer version — see plan.md §5.1.
type FieldType string

const (
	FieldTypeScalar FieldType = "scalar"
	FieldTypeArray  FieldType = "array"
)

// FieldTypeRegistry maps field name -> encoding for one slicer (and,
// optionally, slicer version).
//
// Plan §5.1: registry is "populated by sampling each slicer's system +
// user profiles at startup (or hardcoded table built during development)"
// and "slicer-version-aware if formats change between versions".
type FieldTypeRegistry interface {
	Lookup(field string) (FieldType, bool)
}

// NewFieldTypeRegistry returns a hardcoded registry for the given slicer.
// The table is built from observed fixture data (see testdata/fixtures).
// The version parameter is reserved for future slicer-version-aware
// formatting; currently ignored.
func NewFieldTypeRegistry(slicer SlicerID, version string) FieldTypeRegistry {
	switch slicer {
	case SlicerBambuStudio:
		return &hardcodedRegistry{fields: bsFields}
	case SlicerOrcaSlicer:
		return &hardcodedRegistry{fields: orcaFields}
	default:
		return &hardcodedRegistry{fields: map[string]FieldType{}}
	}
}

type hardcodedRegistry struct {
	fields map[string]FieldType
}

func (r *hardcodedRegistry) Lookup(field string) (FieldType, bool) {
	ft, ok := r.fields[field]
	return ft, ok
}

// bsFields maps every observed BambuStudio field to its on-disk encoding.
// BambuStudio wraps speed/acceleration fields in single-element arrays;
// print_extruder_id and print_extruder_variant are also arrays.
var bsFields = map[string]FieldType{
	"brim_object_gap":             FieldTypeScalar,
	"brim_type":                   FieldTypeScalar,
	"detect_thin_wall":            FieldTypeScalar,
	"elefant_foot_compensation":   FieldTypeScalar,
	"enable_support":              FieldTypeScalar,
	"from":                        FieldTypeScalar,
	"inherits":                    FieldTypeScalar,
	"initial_layer_infill_speed":  FieldTypeArray,
	"initial_layer_speed":         FieldTypeArray,
	"inner_wall_acceleration":     FieldTypeArray,
	"inner_wall_speed":            FieldTypeArray,
	"internal_solid_infill_speed": FieldTypeArray,
	"ironing_flow":                FieldTypeScalar,
	"ironing_spacing":             FieldTypeScalar,
	"ironing_speed":               FieldTypeScalar,
	"name":                        FieldTypeScalar,
	"outer_wall_acceleration":     FieldTypeArray,
	"outer_wall_speed":            FieldTypeArray,
	"overhang_2_4_speed":          FieldTypeArray,
	"overhang_3_4_speed":          FieldTypeArray,
	"overhang_4_4_speed":          FieldTypeArray,
	"print_extruder_id":           FieldTypeArray,
	"print_extruder_variant":      FieldTypeArray,
	"print_settings_id":           FieldTypeScalar,
	"raft_first_layer_expansion":  FieldTypeScalar,
	"resolution":                  FieldTypeScalar,
	"small_perimeter_threshold":   FieldTypeArray,
	"sparse_infill_pattern":       FieldTypeScalar,
	"sparse_infill_speed":         FieldTypeArray,
	"support_bottom_z_distance":   FieldTypeScalar,
	"support_interface_spacing":   FieldTypeScalar,
	"support_top_z_distance":      FieldTypeScalar,
	"support_type":                FieldTypeScalar,
	"top_surface_pattern":         FieldTypeScalar,
	"top_surface_speed":           FieldTypeArray,
	"version":                     FieldTypeScalar,
	"wall_generator":              FieldTypeScalar,
}

// orcaFields maps every observed OrcaSlicer field to its on-disk encoding.
// OrcaSlicer uses scalars for all speed/acceleration fields; only
// print_extruder_id and print_extruder_variant are arrays.
var orcaFields = map[string]FieldType{
	"brim_object_gap":             FieldTypeScalar,
	"brim_type":                   FieldTypeScalar,
	"detect_thin_wall":            FieldTypeScalar,
	"elefant_foot_compensation":   FieldTypeScalar,
	"enable_support":              FieldTypeScalar,
	"from":                        FieldTypeScalar,
	"inherits":                    FieldTypeScalar,
	"initial_layer_infill_speed":  FieldTypeScalar,
	"initial_layer_speed":         FieldTypeScalar,
	"inner_wall_acceleration":     FieldTypeScalar,
	"inner_wall_speed":            FieldTypeScalar,
	"internal_solid_infill_speed": FieldTypeScalar,
	"ironing_flow":                FieldTypeScalar,
	"ironing_spacing":             FieldTypeScalar,
	"ironing_speed":               FieldTypeScalar,
	"name":                        FieldTypeScalar,
	"outer_wall_acceleration":     FieldTypeScalar,
	"outer_wall_speed":            FieldTypeScalar,
	"overhang_2_4_speed":          FieldTypeScalar,
	"overhang_3_4_speed":          FieldTypeScalar,
	"overhang_4_4_speed":          FieldTypeScalar,
	"print_extruder_id":           FieldTypeArray,
	"print_extruder_variant":      FieldTypeArray,
	"print_settings_id":           FieldTypeScalar,
	"raft_first_layer_expansion":  FieldTypeScalar,
	"resolution":                  FieldTypeScalar,
	"small_perimeter_threshold":   FieldTypeScalar,
	"sparse_infill_pattern":       FieldTypeScalar,
	"sparse_infill_speed":         FieldTypeScalar,
	"support_bottom_z_distance":   FieldTypeScalar,
	"support_interface_spacing":   FieldTypeScalar,
	"support_top_z_distance":      FieldTypeScalar,
	"support_type":                FieldTypeScalar,
	"top_surface_pattern":         FieldTypeScalar,
	"top_surface_speed":           FieldTypeScalar,
	"version":                     FieldTypeScalar,
	"wall_generator":              FieldTypeScalar,
}
