package profile

import "testing"

func TestRegistryBSKnownArrayFields(t *testing.T) {
	r := NewFieldTypeRegistry(SlicerBambuStudio, "")
	arrayFields := []string{
		"initial_layer_infill_speed",
		"initial_layer_speed",
		"inner_wall_acceleration",
		"inner_wall_speed",
		"internal_solid_infill_speed",
		"outer_wall_acceleration",
		"outer_wall_speed",
		"overhang_2_4_speed",
		"overhang_3_4_speed",
		"overhang_4_4_speed",
		"small_perimeter_threshold",
		"sparse_infill_speed",
		"top_surface_speed",
		"print_extruder_id",
		"print_extruder_variant",
	}
	for _, f := range arrayFields {
		ft, ok := r.Lookup(f)
		if !ok {
			t.Errorf("Lookup(%q): not found, want found", f)
			continue
		}
		if ft != FieldTypeArray {
			t.Errorf("Lookup(%q) = %v, want %v", f, ft, FieldTypeArray)
		}
	}
}

func TestRegistryBSKnownScalarFields(t *testing.T) {
	r := NewFieldTypeRegistry(SlicerBambuStudio, "")
	scalarFields := []string{
		"brim_object_gap", "brim_type", "detect_thin_wall",
		"elefant_foot_compensation", "enable_support", "from",
		"inherits", "ironing_flow", "ironing_spacing", "ironing_speed",
		"name", "print_settings_id", "raft_first_layer_expansion",
		"resolution", "sparse_infill_pattern", "support_bottom_z_distance",
		"support_interface_spacing", "support_top_z_distance",
		"support_type", "top_surface_pattern", "version", "wall_generator",
	}
	for _, f := range scalarFields {
		ft, ok := r.Lookup(f)
		if !ok {
			t.Errorf("Lookup(%q): not found, want found", f)
			continue
		}
		if ft != FieldTypeScalar {
			t.Errorf("Lookup(%q) = %v, want %v", f, ft, FieldTypeScalar)
		}
	}
}

func TestRegistryOrcaKnownArrayFields(t *testing.T) {
	r := NewFieldTypeRegistry(SlicerOrcaSlicer, "")
	arrayFields := []string{
		"print_extruder_id",
		"print_extruder_variant",
	}
	for _, f := range arrayFields {
		ft, ok := r.Lookup(f)
		if !ok {
			t.Errorf("Lookup(%q): not found, want found", f)
			continue
		}
		if ft != FieldTypeArray {
			t.Errorf("Lookup(%q) = %v, want %v", f, ft, FieldTypeArray)
		}
	}
}

func TestRegistryOrcaKnownScalarFields(t *testing.T) {
	r := NewFieldTypeRegistry(SlicerOrcaSlicer, "")
	// These are array in BS but scalar in Orca.
	scalarInOrca := []string{
		"inner_wall_speed", "outer_wall_speed", "initial_layer_speed",
		"sparse_infill_speed", "top_surface_speed", "inner_wall_acceleration",
		"outer_wall_acceleration", "small_perimeter_threshold",
		"initial_layer_infill_speed", "internal_solid_infill_speed",
		"overhang_2_4_speed", "overhang_3_4_speed", "overhang_4_4_speed",
	}
	for _, f := range scalarInOrca {
		ft, ok := r.Lookup(f)
		if !ok {
			t.Errorf("Lookup(%q): not found, want found", f)
			continue
		}
		if ft != FieldTypeScalar {
			t.Errorf("Lookup(%q) = %v, want %v (scalar in Orca)", f, ft, FieldTypeScalar)
		}
	}
}

func TestRegistryUnknownField(t *testing.T) {
	r := NewFieldTypeRegistry(SlicerBambuStudio, "")
	unknownFields := []string{
		"nonexistent_field", "custom_slicer_opt", "", "xyz_123",
	}
	for _, f := range unknownFields {
		ft, ok := r.Lookup(f)
		if ok {
			t.Errorf("Lookup(%q): found (%v), want not found", f, ft)
		}
	}
}

func TestRegistryOrcaUnknownField(t *testing.T) {
	r := NewFieldTypeRegistry(SlicerOrcaSlicer, "")
	if _, ok := r.Lookup("nonexistent_field"); ok {
		t.Error("unknown field should not be found in Orca registry")
	}
}

func TestRegistryDefaultEmptyForUnknownSlicer(t *testing.T) {
	r := NewFieldTypeRegistry(SlicerID("unknown"), "")
	if _, ok := r.Lookup("inner_wall_speed"); ok {
		t.Error("unknown slicer should have empty registry")
	}
}

func TestDenormalizeArray(t *testing.T) {
	// Scalar -> single-element array.
	got := denormalizeArray("400")
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("denormalizeArray(string) returned %T, want []any", got)
	}
	if len(arr) != 1 || arr[0] != "400" {
		t.Errorf("denormalizeArray(\"400\") = %v, want [\"400\"]", arr)
	}

	// Already an array — returned as-is.
	input := []any{"a", "b"}
	got = denormalizeArray(input)
	arr, ok = got.([]any)
	if !ok {
		t.Fatalf("denormalizeArray([]any) returned %T, want []any", got)
	}
	if len(arr) != 2 || arr[0] != "a" || arr[1] != "b" {
		t.Errorf("denormalizeArray(%v) = %v, want %v", input, arr, input)
	}

	// Number scalar.
	got = denormalizeArray(float64(42))
	arr, ok = got.([]any)
	if !ok {
		t.Fatalf("denormalizeArray(float64) returned %T, want []any", got)
	}
	if len(arr) != 1 || arr[0] != float64(42) {
		t.Errorf("denormalizeArray(42) = %v, want [42]", arr)
	}
}
